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
	"net"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/rs/zerolog"
	"golang.org/x/sys/unix"

	"github.com/pawelgaczynski/gain/iouring"
	"github.com/pawelgaczynski/gain/logger"
)

type Server struct {
	config        Config
	closer        func() error
	closeChan     chan closeSignal
	closeBackChan chan bool
	running       bool
	logger        zerolog.Logger

	startListener func()
	eventHandler  EventHandler

	readWriteWorkers []readWriteWorker
}

type closeSignal int

const (
	system closeSignal = iota
	user
)

func (s *Server) SetStartListener(startListener func()) {
	s.startListener = startListener
}

func (s *Server) startReactor(socket int, supportedFeatures SupportedFeatures) error {
	var doneWg, startedWg sync.WaitGroup
	loadBalancer, err := createLoadBalancer(s.config.loadBalancing)
	if err != nil {
		return err
	}
	acceptor, err := newAcceptorWorker(acceptorWorkerConfig{
		workerConfig: workerConfig{
			lockOSThread:    s.config.lockOSThread,
			processPriority: s.config.processPriority,
			bufferSize:      s.config.bufferSize,
			maxCQEvents:     defaultMaxCQEvents,
			batchSubmitter:  s.config.batchSubmitter,
			loggerLevel:     s.config.loggerLevel,
			prettyLogger:    s.config.prettyLogger,
		},
	}, loadBalancer, s.eventHandler, supportedFeatures)
	if err != nil {
		return err
	}
	acceptor.startListener = func() {
		startedWg.Done()
	}
	var consumers sync.Map
	for i := 0; i < s.config.workers; i++ {
		consumer, err := newConsumerWorker(i+1, consumerConfig{
			readWriteWorkerConfig: readWriteWorkerConfig{
				workerConfig: workerConfig{
					lockOSThread:          s.config.lockOSThread,
					bufferSize:            s.config.bufferSize,
					maxCQEvents:           defaultMaxCQEvents,
					batchSubmitter:        s.config.batchSubmitter,
					loggerLevel:           s.config.loggerLevel,
					prettyLogger:          s.config.prettyLogger,
					prefillConnectionPool: s.config.prefillConnectionPool,
				},
				asyncHandler:  s.config.asyncHandler,
				goroutinePool: s.config.goroutinePool,
				maxConn:       s.config.maxConn,
			},
		}, s.eventHandler, supportedFeatures)
		if err != nil {
			return err
		}
		consumer.startListener = func() {
			startedWg.Done()
		}
		acceptor.registerConsumer(consumer)
		s.readWriteWorkers = append(s.readWriteWorkers, consumer)
		consumers.Store(i, consumer)
	}
	s.closer = func() error {
		acceptor.shutdown()
		return nil
	}
	numberOfWorkers := int32(s.config.workers)
	consumers.Range(func(key any, value any) bool {
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
		go func(c *consumerWorker) {
			err = c.loop(socket)
			if err != nil {
				atomic.AddInt32(&numberOfWorkers, -1)
				livingWorkers := atomic.LoadInt32(&numberOfWorkers)
				s.logger.Error().Err(err).Int32("living workers", livingWorkers).Int("worker index", index).Msg("Worker died...")
				if !c.started() {
					startedWg.Done()
				}
				consumers.Delete(index)
			}
			doneWg.Done()
		}(worker)
		<-worker.startedChan
		return true
	})
	startedWg.Add(1)
	doneWg.Add(1)
	go func(acceptor *acceptorWorker) {
		err = acceptor.loop(socket)
		if err != nil {
			log.Panic(err)
		}
		doneWg.Done()
	}(acceptor)
	startedWg.Wait()
	s.running = true
	if s.startListener != nil {
		s.startListener()
	}
	doneWg.Wait()
	return nil
}

func (s *Server) startSharding(sockets []int, supportedFeatures SupportedFeatures) error {
	var doneWg, startedWg sync.WaitGroup
	var workers sync.Map
	for i := 0; i < s.config.workers; i++ {
		shardWorker, err := newShardWorker(i, shardWorkerConfig{
			readWriteWorkerConfig: readWriteWorkerConfig{
				workerConfig: workerConfig{
					lockOSThread:          s.config.lockOSThread,
					bufferSize:            s.config.bufferSize,
					maxCQEvents:           defaultMaxCQEvents,
					batchSubmitter:        s.config.batchSubmitter,
					loggerLevel:           s.config.loggerLevel,
					prettyLogger:          s.config.prettyLogger,
					prefillConnectionPool: s.config.prefillConnectionPool,
				},
				asyncHandler:  s.config.asyncHandler,
				goroutinePool: s.config.goroutinePool,
				maxConn:       s.config.maxConn,
			},
		}, s.eventHandler)
		if err != nil {
			return err
		}
		shardWorker.startListener = func() {
			startedWg.Done()
		}
		s.readWriteWorkers = append(s.readWriteWorkers, shardWorker)
		workers.Store(i, shardWorker)
	}
	s.closer = func() error {
		workers.Range(func(key any, value any) bool {
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
	numberOfWorkers := int32(s.config.workers)
	workers.Range(func(key any, value any) bool {
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
		go func(w *shardWorker) {
			err := w.loop(sockets[index])
			if err != nil {
				atomic.AddInt32(&numberOfWorkers, -1)
				livingWorkers := atomic.LoadInt32(&numberOfWorkers)
				s.logger.Error().Err(err).Int32("living workers", livingWorkers).Int("worker index", index).Msg("Worker died...")
				if !w.started() {
					startedWg.Done()
				}
				workers.Delete(index)
			}
			doneWg.Done()
		}(worker)
		<-worker.startedChan
		return true
	})
	startedWg.Wait()
	s.running = true
	if s.startListener != nil {
		s.startListener()
	}
	doneWg.Wait()
	return nil
}

func (s *Server) setupSocket() (int, error) {
	var err error
	var socketFd int
	var ipAddr [4]byte
	defer func() {
		if err != nil {
			_ = syscall.Shutdown(socketFd, syscall.SHUT_RDWR)
		}
	}()
	socketFd, err = syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return -1, fmt.Errorf("could not open socket")
	}
	netAddr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:80")
	if err != nil {
		return -1, fmt.Errorf("could not open socket")
	}
	copy(ipAddr[:], netAddr.IP)
	if err != nil {
		return -1, err
	}
	err = syscall.SetsockoptInt(socketFd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	if err != nil {
		return -1, err
	}
	err = unix.SetsockoptInt(socketFd, unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
	if err != nil {
		return -1, err
	}
	err = unix.SetsockoptInt(socketFd, unix.IPPROTO_TCP, unix.TCP_NODELAY, 1)
	if err != nil {
		return -1, err
	}
	err = unix.SetsockoptInt(socketFd, unix.IPPROTO_TCP, unix.TCP_QUICKACK, 1)
	if err != nil {
		return -1, err
	}
	err = unix.SetsockoptInt(socketFd, unix.IPPROTO_TCP, unix.TCP_FASTOPEN, int(s.config.maxConn))
	if err != nil {
		return -1, err
	}
	err = unix.SetsockoptInt(socketFd, unix.SOL_SOCKET, unix.SO_RCVBUF, maxSocketReceiveBuffer)
	if err != nil {
		return -1, err
	}
	err = unix.SetsockoptInt(socketFd, unix.SOL_SOCKET, unix.SO_SNDBUF, maxSocketSendBuffer)
	if err != nil {
		return -1, err
	}
	sa := &syscall.SockaddrInet4{
		Port: s.config.port,
		Addr: ipAddr,
	}
	err = syscall.Bind(socketFd, sa)
	if err != nil {
		return -1, err
	}
	err = syscall.Listen(socketFd, int(s.config.maxConn))
	if err != nil {
		return -1, err
	}
	if s.config.cbpf {
		err = newFilter(uint32(s.config.workers)).applyTo(socketFd)
		if err != nil {
			return -1, err
		}
	}
	return socketFd, nil
}

func (s *Server) start(mainProcess bool) error {
	var err error
	supportedFeatures := SupportedFeatures{}
	supportedFeatures.ringsMessaging, err = iouring.IsOpSupported(iouring.OpMsgRing)
	if err != nil {
		return err
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
		err := s.closer()
		if err != nil {
			s.logger.Error().Err(err).Msg("Closing server error")
		}
		s.running = false
		s.closeBackChan <- true
		if closeSig == system {
			os.Exit(0)
		}
	}()
	if s.config.sharding {
		sockets := make([]int, s.config.workers)
		for i := 0; i < len(sockets); i++ {
			socket, err := s.setupSocket()
			if err != nil {
				return err
			}
			sockets[i] = socket
		}
		return s.startSharding(sockets, supportedFeatures)
	}
	socket, err := s.setupSocket()
	if err != nil {
		return err
	}
	return s.startReactor(socket, supportedFeatures)
}

func (s *Server) StartAsMainProcess() error {
	return s.start(true)
}

func (s *Server) Start() error {
	return s.start(false)
}

func (s *Server) Close() {
	s.closeChan <- user
	<-s.closeBackChan
}

func (s *Server) ActiveConnections() int {
	connections := 0
	for index := range s.readWriteWorkers {
		connections += s.readWriteWorkers[index].activeConnections()
	}
	return connections
}

func (s *Server) IsRunning() bool {
	return s.running
}

func NewServer(config Config, eventHandler EventHandler) *Server {
	return &Server{
		config:           config,
		logger:           logger.NewLogger("server", config.loggerLevel, config.prettyLogger),
		eventHandler:     eventHandler,
		closeChan:        make(chan closeSignal),
		closeBackChan:    make(chan bool),
		readWriteWorkers: make([]readWriteWorker, 0),
	}
}
