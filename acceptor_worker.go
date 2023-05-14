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
	"net"
	"sync/atomic"
	"time"

	"github.com/pawelgaczynski/gain/iouring"
	"github.com/pawelgaczynski/gain/logger"
	"github.com/pawelgaczynski/gain/pkg/socket"
)

type acceptorWorkerConfig struct {
	workerConfig
	tcpKeepAlive time.Duration
}

type acceptorWorker struct {
	*acceptor
	*workerImpl
	config        acceptorWorkerConfig
	ring          *iouring.Ring
	loadBalancer  loadBalancer
	eventHandler  EventHandler
	addConnMethod func(consumer, int32) error

	accepting atomic.Bool
}

func (a *acceptorWorker) addConnViaRing(worker consumer, fileDescriptor int32) error {
	entry, err := a.ring.GetSQE()
	if err != nil {
		return fmt.Errorf("error getting SQE: %w", err)
	}

	entry.PrepareMsgRing(worker.ringFd(), uint32(fileDescriptor), addConnFlag, 0)
	entry.UserData = addConnFlag

	return nil
}

func (a *acceptorWorker) addConnViaQueue(worker consumer, fileDescriptor int32) error {
	err := worker.addConnToQueue(int(fileDescriptor))
	if err != nil {
		return fmt.Errorf("error adding connection to queue: %w", err)
	}

	return nil
}

func (a *acceptorWorker) registerConsumer(consumer consumer) {
	a.loadBalancer.register(consumer)
}

func (a *acceptorWorker) closeRingAndConsumers() {
	_ = a.syscallCloseSocket(a.fd)
	a.accepting.Store(false)

	err := a.loadBalancer.forEach(func(w consumer) error {
		w.shutdown()

		return nil
	})
	if err != nil {
		a.logError(err).Msg("Closing consumers error")
	}
}

func (a *acceptorWorker) loop(fd int) error {
	a.logInfo().Int("fd", fd).Msg("Starting acceptor loop...")
	a.fd = fd
	a.prepareHandler = func() error {
		err := a.addAcceptRequest()
		if err == nil {
			a.accepting.Store(true)
		}

		return err
	}
	a.shutdownHandler = func() bool {
		if a.needToShutdown() {
			a.onCloseHandler()
			a.markShutdownInProgress()

			return false
		}

		return true
	}
	err := a.startLoop(0, func(cqe *iouring.CompletionQueueEvent) error {
		if cqe.UserData()&addConnFlag > 0 {
			return nil
		}
		err := a.addAcceptRequest()
		if err != nil {
			a.logError(err).
				Int("fd", fd).
				Msg("Add accept() request error")

			return err
		}

		if exit := a.processEvent(cqe, func(cqe *iouring.CompletionQueueEvent) bool {
			return !a.accepting.Load()
		}); exit {
			return nil
		}

		if a.config.tcpKeepAlive > 0 {
			err = socket.SetKeepAlivePeriod(int(cqe.Res()), int(a.config.tcpKeepAlive.Seconds()))
			if err != nil {
				return fmt.Errorf("fd: %d, setting tcpKeepAlive error: %w", cqe.Res(), err)
			}
		}
		var clientAddr net.Addr
		clientAddr, err = a.lastClientAddr()

		if err != nil {
			a.logError(err).
				Int("fd", fd).
				Int32("conn fd", cqe.Res()).
				Msg("Getting client address error")
			_ = a.syscallCloseSocket(int(cqe.Res()))

			return nil
		}

		nextConsumer := a.loadBalancer.next(clientAddr)
		a.logDebug().
			Int("fd", fd).
			Int32("conn fd", cqe.Res()).
			Int("consumer", nextConsumer.index()).
			Msg("Forwarding accepted connection to consumer")

		nextConsumer.setSocketAddr(int(cqe.Res()), clientAddr)

		err = a.addConnMethod(nextConsumer, cqe.Res())
		if err != nil {
			a.logError(err).
				Int("fd", fd).
				Int32("conn fd", cqe.Res()).
				Msg("Add connection to consumer error")

			_ = a.syscallCloseSocket(int(cqe.Res()))

			return nil
		}

		return nil
	})

	a.notifyFinish()

	return err
}

func newAcceptorWorker(
	config acceptorWorkerConfig, loadBalancer loadBalancer, eventHandler EventHandler, features supportedFeatures,
) (*acceptorWorker, error) {
	ring, err := iouring.CreateRing()
	if err != nil {
		return nil, fmt.Errorf("creating ring error: %w", err)
	}
	logger := logger.NewLogger("acceptor", config.loggerLevel, config.prettyLogger)
	connectionManager := newConnectionManager()
	acceptor := &acceptorWorker{
		workerImpl:   newWorkerImpl(ring, config.workerConfig, 0, logger),
		acceptor:     newAcceptor(ring, connectionManager),
		config:       config,
		ring:         ring,
		eventHandler: eventHandler,
		loadBalancer: loadBalancer,
	}

	if features.ringsMessaging {
		acceptor.addConnMethod = acceptor.addConnViaRing
	} else {
		acceptor.addConnMethod = acceptor.addConnViaQueue
	}
	acceptor.onCloseHandler = acceptor.closeRingAndConsumers

	return acceptor, nil
}
