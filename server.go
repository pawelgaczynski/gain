// Copyright (c) 2023 Paweł Gaczyński
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gain

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/pawelgaczynski/gain/iouring"
	"github.com/pawelgaczynski/gain/logger"
	gainErrors "github.com/pawelgaczynski/gain/pkg/errors"
	gainNet "github.com/pawelgaczynski/gain/pkg/net"
	"github.com/rs/zerolog"
)

const (
	inactive uint32 = iota
	starting
	running
	closing
)

type Server interface {
	// Start starts a server that will listen on the specified address and
	// waits for SIGTERM or SIGINT signals to close the server.
	StartAsMainProcess(address string) error
	// Start starts a server that will listen on the specified address.
	Start(address string) error
	// Shutdown closes all connections and shuts down server. It's blocking until the server shuts down.
	Shutdown()
	// AsyncShutdown closes all connections and shuts down server in asynchronous manner.
	// It does not wait for the server shutdown to complete.
	AsyncShutdown()
	// ActiveConnections returns the number of active connections.
	ActiveConnections() int
	// IsRunning returns true if server is running and handling requests.
	IsRunning() bool
}

type engine struct {
	config  Config
	network string
	address string

	closer        func() error
	closeChan     chan closeSignal
	closeBackChan chan bool
	state         atomic.Uint32
	logger        zerolog.Logger

	eventHandler EventHandler

	readWriteWorkers sync.Map
}

type closeSignal int

const (
	system closeSignal = iota
	user
)

func (e *engine) startConsumers(startedWg, doneWg *sync.WaitGroup) {
	numberOfWorkers := int32(e.config.Workers)
	e.readWriteWorkers.Range(func(key any, value any) bool {
		var index int
		var worker *consumerWorker
		var ok bool
		if index, ok = key.(int); !ok {
			return false
		}
		if worker, ok = value.(*consumerWorker); !ok {
			return false
		}
		startedWg.Add(1)
		doneWg.Add(1)
		go func(cWorker *consumerWorker) {
			err := cWorker.loop(0)
			if err != nil {
				e.handleWorkerStop(index, &numberOfWorkers, err, startedWg, *cWorker.readWriteWorkerImpl)
			}
			doneWg.Done()
		}(worker)
		<-worker.startedChan

		return true
	})
}

func (e *engine) handleWorkerStop(index int, numberOfWorkers *int32, err error, startedWg *sync.WaitGroup, worker readWriteWorkerImpl) {
	atomic.AddInt32(numberOfWorkers, -1)
	livingWorkers := atomic.LoadInt32(numberOfWorkers)
	e.logger.Error().Err(err).Int32("living workers", livingWorkers).Int("worker index", index).Msg("Worker died...")
	if !worker.started() {
		startedWg.Done()
	}
	e.readWriteWorkers.Delete(index)
}

func (e *engine) startReactor(listener *listener, features supportedFeatures) error {
	var doneWg, startedWg sync.WaitGroup

	lb, err := createLoadBalancer(e.config.LoadBalancing)
	if err != nil {
		return err
	}

	acceptor, err := newAcceptorWorker(acceptorWorkerConfig{
		workerConfig: workerConfig{
			cpuAffinity:     e.config.CPUAffinity,
			processPriority: e.config.ProcessPriority,
			maxCQEvents:     defaultMaxCQEvents,
			loggerLevel:     e.config.LoggerLevel,
			prettyLogger:    e.config.PrettyLogger,
		},
		tcpKeepAlive: e.config.TCPKeepAlive,
	}, lb, e.eventHandler, features)
	if err != nil {
		return err
	}

	acceptor.startListener = func() {
		startedWg.Done()
	}

	for i := 0; i < e.config.Workers; i++ {
		var consumer *consumerWorker

		consumer, err = newConsumerWorker(i+1, listener.addr, consumerConfig{
			readWriteWorkerConfig: readWriteWorkerConfig{
				workerConfig: workerConfig{
					cpuAffinity:  e.config.CPUAffinity,
					maxCQEvents:  defaultMaxCQEvents,
					loggerLevel:  e.config.LoggerLevel,
					prettyLogger: e.config.PrettyLogger,
				},
				asyncHandler:  e.config.AsyncHandler,
				goroutinePool: e.config.GoroutinePool,
			},
		}, e.eventHandler, features)
		if err != nil {
			return err
		}
		consumer.startListener = func() {
			startedWg.Done()
		}
		acceptor.registerConsumer(consumer)
		e.readWriteWorkers.Store(i, consumer)
	}
	e.closer = func() error {
		acceptor.shutdown()

		return nil
	}

	e.startConsumers(&startedWg, &doneWg)

	startedWg.Add(1)
	doneWg.Add(1)

	go func(acceptor *acceptorWorker) {
		err = acceptor.loop(listener.fd)
		if err != nil {
			log.Panic(err)
		}

		doneWg.Done()
	}(acceptor)
	startedWg.Wait()
	e.state.Store(running)

	e.eventHandler.OnStart(e)
	doneWg.Wait()

	return nil
}

func (e *engine) startSocketSharding(listeners []*listener, protocol string) error {
	var (
		doneWg    sync.WaitGroup
		startedWg sync.WaitGroup
	)

	for i := 0; i < e.config.Workers; i++ {
		shardWorker, err := newShardWorker(i, listeners[i].addr, shardWorkerConfig{
			readWriteWorkerConfig: readWriteWorkerConfig{
				workerConfig: workerConfig{
					cpuAffinity:  e.config.CPUAffinity,
					maxCQEvents:  defaultMaxCQEvents,
					loggerLevel:  e.config.LoggerLevel,
					prettyLogger: e.config.PrettyLogger,
				},
				asyncHandler:  e.config.AsyncHandler,
				goroutinePool: e.config.GoroutinePool,
				sendRecvMsg:   protocol == gainNet.UDP,
			},
			tcpKeepAlive: e.config.TCPKeepAlive,
		}, e.eventHandler)
		if err != nil {
			return err
		}
		shardWorker.startListener = func() {
			startedWg.Done()
		}
		e.readWriteWorkers.Store(i, shardWorker)
	}
	e.closer = func() error {
		e.readWriteWorkers.Range(func(key any, value any) bool {
			var worker *shardWorker
			var ok bool
			if worker, ok = value.(*shardWorker); !ok {
				return false
			}
			worker.shutdown()
			e.logger.Warn().Msgf("Worker %d closed", worker.index())

			return true
		})

		return nil
	}
	numberOfWorkers := int32(e.config.Workers)
	e.readWriteWorkers.Range(func(key any, value any) bool {
		var index int
		var worker *shardWorker
		var ok bool
		if index, ok = key.(int); !ok {
			return false
		}
		if worker, ok = value.(*shardWorker); !ok {
			return false
		}
		startedWg.Add(1)
		doneWg.Add(1)
		go func(sWorker *shardWorker) {
			err := sWorker.loop(listeners[index].fd)
			if err != nil {
				e.handleWorkerStop(index, &numberOfWorkers, err, &startedWg, *sWorker.readWriteWorkerImpl)
			}
			doneWg.Done()
		}(worker)
		<-worker.startedChan

		return true
	})
	startedWg.Wait()
	e.state.Store(running)

	e.eventHandler.OnStart(e)
	doneWg.Wait()

	return nil
}

func (e *engine) start(mainProcess bool, address string) error {
	if state := e.state.Load(); state != inactive {
		return gainErrors.ErrInvalidState
	}

	e.state.Store(starting)

	var (
		err      error
		features = supportedFeatures{}
	)

	features.ringsMessaging, err = iouring.IsOpSupported(iouring.OpMsgRing)
	if err != nil {
		return fmt.Errorf("isOpSupported error: %w", err)
	}

	if mainProcess {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			<-sigs
			e.closeChan <- system
		}()
	}

	go func() {
		closeSig := <-e.closeChan

		closeErr := e.closer()
		if closeErr != nil {
			e.logger.Error().Err(closeErr).Msg("Closing server error")
		}

		e.state.Store(inactive)

		e.closeBackChan <- true

		if closeSig == system {
			os.Exit(0)
		}
	}()

	e.network, e.address = parseProtoAddr(address)

	if e.config.Architecture == SocketSharding || e.network == gainNet.UDP {
		listeners := make([]*listener, e.config.Workers)
		for i := 0; i < len(listeners); i++ {
			var listener *listener

			listener, err = initListener(e.network, e.address, e.config)
			if err != nil {
				return err
			}
			listeners[i] = listener
		}

		return e.startSocketSharding(listeners, e.network)
	}

	listener, err := initListener(e.network, e.address, e.config)
	if err != nil {
		return err
	}

	return e.startReactor(listener, features)
}

func (e *engine) StartAsMainProcess(address string) error {
	if e.IsRunning() {
		return gainErrors.ErrServerAlreadyRunning
	}

	return e.start(true, address)
}

func (e *engine) Start(address string) error {
	if e.IsRunning() {
		return gainErrors.ErrServerAlreadyRunning
	}

	return e.start(false, address)
}

func (e *engine) Shutdown() {
	if state := e.state.Load(); state == running {
		e.state.Store(closing)
		e.closeChan <- user
		<-e.closeBackChan
	}
}

func (e *engine) AsyncShutdown() {
	if state := e.state.Load(); state == running {
		e.state.Store(closing)
		e.closeChan <- user
	}
}

func (e *engine) ActiveConnections() int {
	connections := 0

	e.readWriteWorkers.Range(func(key any, value any) bool {
		if worker, ok := value.(readWriteWorker); ok {
			connections += worker.activeConnections()

			return true
		}

		return false
	})

	return connections
}

func (e *engine) IsRunning() bool {
	return e.state.Load() == running
}

func NewServer(eventHandler EventHandler, config Config) Server {
	return &engine{
		config:        config,
		logger:        logger.NewLogger("server", config.LoggerLevel, config.PrettyLogger),
		eventHandler:  eventHandler,
		closeChan:     make(chan closeSignal),
		closeBackChan: make(chan bool),
	}
}
