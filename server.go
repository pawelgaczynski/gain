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

type Server struct {
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

func (s *Server) startConsumers(startedWg, doneWg *sync.WaitGroup) {
	numberOfWorkers := int32(s.config.Workers)
	s.readWriteWorkers.Range(func(key any, value any) bool {
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
				atomic.AddInt32(&numberOfWorkers, -1)
				livingWorkers := atomic.LoadInt32(&numberOfWorkers)
				s.logger.Error().Err(err).Int32("living workers", livingWorkers).Int("worker index", index).Msg("Worker died...")
				if !cWorker.started() {
					startedWg.Done()
				}
				s.readWriteWorkers.Delete(index)
			}
			doneWg.Done()
		}(worker)
		<-worker.startedChan

		return true
	})
}

func (s *Server) startReactor(listener *listener, features supportedFeatures) error {
	var doneWg, startedWg sync.WaitGroup

	lb, err := createLoadBalancer(s.config.LoadBalancing)
	if err != nil {
		return err
	}

	acceptor, err := newAcceptorWorker(acceptorWorkerConfig{
		workerConfig: workerConfig{
			cpuAffinity:     s.config.CPUAffinity,
			processPriority: s.config.ProcessPriority,
			maxCQEvents:     defaultMaxCQEvents,
			loggerLevel:     s.config.LoggerLevel,
			prettyLogger:    s.config.PrettyLogger,
		},
		tcpKeepAlive: s.config.TCPKeepAlive,
	}, lb, s.eventHandler, features)
	if err != nil {
		return err
	}

	acceptor.startListener = func() {
		startedWg.Done()
	}

	for i := 0; i < s.config.Workers; i++ {
		var consumer *consumerWorker

		consumer, err = newConsumerWorker(i+1, listener.addr, consumerConfig{
			readWriteWorkerConfig: readWriteWorkerConfig{
				workerConfig: workerConfig{
					cpuAffinity:  s.config.CPUAffinity,
					maxCQEvents:  defaultMaxCQEvents,
					loggerLevel:  s.config.LoggerLevel,
					prettyLogger: s.config.PrettyLogger,
				},
				asyncHandler:  s.config.AsyncHandler,
				goroutinePool: s.config.GoroutinePool,
			},
		}, s.eventHandler, features)
		if err != nil {
			return err
		}
		consumer.startListener = func() {
			startedWg.Done()
		}
		acceptor.registerConsumer(consumer)
		s.readWriteWorkers.Store(i, consumer)
	}
	s.closer = func() error {
		acceptor.shutdown()

		return nil
	}

	s.startConsumers(&startedWg, &doneWg)

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
	s.state.Store(running)

	s.eventHandler.OnStart(s)
	doneWg.Wait()

	return nil
}

func (s *Server) startSocketSharding(listeners []*listener, protocol string) error {
	var (
		doneWg    sync.WaitGroup
		startedWg sync.WaitGroup
	)

	for i := 0; i < s.config.Workers; i++ {
		shardWorker, err := newShardWorker(i, listeners[i].addr, shardWorkerConfig{
			readWriteWorkerConfig: readWriteWorkerConfig{
				workerConfig: workerConfig{
					cpuAffinity:  s.config.CPUAffinity,
					maxCQEvents:  defaultMaxCQEvents,
					loggerLevel:  s.config.LoggerLevel,
					prettyLogger: s.config.PrettyLogger,
				},
				asyncHandler:  s.config.AsyncHandler,
				goroutinePool: s.config.GoroutinePool,
				sendRecvMsg:   protocol == gainNet.UDP,
			},
			tcpKeepAlive: s.config.TCPKeepAlive,
		}, s.eventHandler)
		if err != nil {
			return err
		}
		shardWorker.startListener = func() {
			startedWg.Done()
		}
		s.readWriteWorkers.Store(i, shardWorker)
	}
	s.closer = func() error {
		s.readWriteWorkers.Range(func(key any, value any) bool {
			var worker *shardWorker
			var ok bool
			if worker, ok = value.(*shardWorker); !ok {
				return false
			}
			worker.shutdown()
			s.logger.Warn().Msgf("Worker %d closed", worker.index())

			return true
		})

		return nil
	}
	numberOfWorkers := int32(s.config.Workers)
	s.readWriteWorkers.Range(func(key any, value any) bool {
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
				atomic.AddInt32(&numberOfWorkers, -1)
				livingWorkers := atomic.LoadInt32(&numberOfWorkers)
				s.logger.Error().Err(err).Int32("living workers", livingWorkers).Int("worker index", index).Msg("Worker died...")
				if !sWorker.started() {
					startedWg.Done()
				}
				s.readWriteWorkers.Delete(index)
			}
			doneWg.Done()
		}(worker)
		<-worker.startedChan

		return true
	})
	startedWg.Wait()
	s.state.Store(running)

	s.eventHandler.OnStart(s)
	doneWg.Wait()

	return nil
}

func (s *Server) start(mainProcess bool, address string) error {
	if state := s.state.Load(); state != inactive {
		return gainErrors.ErrInvalidState
	}

	s.state.Store(starting)

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
			s.closeChan <- system
		}()
	}

	go func() {
		closeSig := <-s.closeChan

		closeErr := s.closer()
		if closeErr != nil {
			s.logger.Error().Err(closeErr).Msg("Closing server error")
		}

		s.state.Store(inactive)

		s.closeBackChan <- true

		if closeSig == system {
			os.Exit(0)
		}
	}()

	s.network, s.address = parseProtoAddr(address)

	if s.config.Architecture == SocketSharding || s.network == gainNet.UDP {
		listeners := make([]*listener, s.config.Workers)
		for i := 0; i < len(listeners); i++ {
			var listener *listener

			listener, err = initListener(s.network, s.address, s.config)
			if err != nil {
				return err
			}
			listeners[i] = listener
		}

		return s.startSocketSharding(listeners, s.network)
	}

	listener, err := initListener(s.network, s.address, s.config)
	if err != nil {
		return err
	}

	return s.startReactor(listener, features)
}

// Start starts a server that will listen on the specified address and
// waits for SIGTERM or SIGINT signals to close the server.
func (s *Server) StartAsMainProcess(address string) error {
	return s.start(true, address)
}

// Start starts a server that will listen on the specified address.
func (s *Server) Start(address string) error {
	return s.start(false, address)
}

// Close closes all connections and server.
func (s *Server) Close() {
	if state := s.state.Load(); state == running {
		s.state.Store(closing)
		s.closeChan <- user
		<-s.closeBackChan
	}
}

// ActiveConnections returns the number of active connections.
func (s *Server) ActiveConnections() int {
	connections := 0

	s.readWriteWorkers.Range(func(key any, value any) bool {
		if worker, ok := value.(readWriteWorker); ok {
			connections += worker.activeConnections()

			return true
		}

		return false
	})

	return connections
}

// IsRunning returns true if server is running and handling requests.
func (s *Server) IsRunning() bool {
	return s.state.Load() == running
}

func NewServer(eventHandler EventHandler, config Config) *Server {
	return &Server{
		config:        config,
		logger:        logger.NewLogger("server", config.LoggerLevel, config.PrettyLogger),
		eventHandler:  eventHandler,
		closeChan:     make(chan closeSignal),
		closeBackChan: make(chan bool),
	}
}
