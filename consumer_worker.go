package gain

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

import (
	"fmt"
	"net"
	"sync"
	"syscall"

	"github.com/pawelgaczynski/gain/logger"
	gainErrors "github.com/pawelgaczynski/gain/pkg/errors"
	"github.com/pawelgaczynski/gain/pkg/queue"
	"github.com/pawelgaczynski/giouring"
)

type consumerConfig struct {
	readWriteWorkerConfig
}

type consumer interface {
	readWriteWorker
	addConnToQueue(fd int) error
	setSocketAddr(fd int, addr net.Addr)
}

type consumerWorker struct {
	*readWriteWorkerImpl
	config consumerConfig

	socketAddresses sync.Map
	// used for kernels < 5.18 where OP_MSG_RING is not supported
	connQueue queue.LockFreeQueue[int]
}

func (c *consumerWorker) setSocketAddr(fd int, addr net.Addr) {
	c.socketAddresses.Store(fd, addr)
}

func (c *consumerWorker) addConnToQueue(fd int) error {
	if c.connQueue == nil {
		return gainErrors.ErrConnectionQueueIsNil
	}

	c.connQueue.Enqueue(fd)

	return nil
}

func (c *consumerWorker) closeAllConns() {
	c.logWarn().Msg("Closing connections")
	c.connectionManager.close(func(conn *connection) bool {
		err := c.addCloseConnRequest(conn)
		if err != nil {
			c.logError(err).Msg("Add close() connection request error")
		}

		return err == nil
	}, -1)
}

func (c *consumerWorker) activeConnections() int {
	return c.connectionManager.activeConnections()
}

func (c *consumerWorker) handleConn(conn *connection, cqe *giouring.CompletionQueueEvent) {
	var (
		err    error
		errMsg string
	)

	switch conn.state {
	case connRead:
		err = c.onRead(cqe, conn)
		if err != nil {
			errMsg = "read error"
		}

	case connWrite:
		n := int(cqe.Res)
		conn.onKernelWrite(n)
		c.logDebug().Int("fd", conn.fd).Int32("count", cqe.Res).Msg("Bytes writed")

		conn.setUserSpace()
		c.eventHandler.OnWrite(conn, n)

		err = c.addNextRequest(conn)
		if err != nil {
			errMsg = "add request error"
		}

	case connClose:
		if cqe.UserData&closeConnFlag > 0 {
			c.closeConn(conn, false, nil)
		} else if cqe.UserData&writeDataFlag > 0 {
			n := int(cqe.Res)
			conn.onKernelWrite(n)
			c.logDebug().Int("fd", conn.fd).Int32("count", cqe.Res).Msg("Bytes writed")
			conn.setUserSpace()
			c.eventHandler.OnWrite(conn, n)
		}

	default:
		err = gainErrors.ErrorUnknownConnectionState(int(conn.state))
	}

	if err != nil {
		c.logError(err).Msg(errMsg)
		c.closeConn(conn, true, err)
	}
}

func (c *consumerWorker) handleNewConn(fd int) error {
	conn := c.connectionManager.getFd(fd)
	conn.fd = fd
	conn.localAddr = c.localAddr

	if remoteAddr, ok := c.socketAddresses.Load(fd); ok {
		conn.remoteAddr, _ = remoteAddr.(net.Addr)

		c.socketAddresses.Delete(fd)
	} else {
		c.logError(gainErrors.ErrorAddressNotFound(fd)).Msg("Get connection address error")
	}

	conn.setUserSpace()
	c.eventHandler.OnAccept(conn)

	return c.addNextRequest(conn)
}

func (c *consumerWorker) getConnsFromQueue() {
	for {
		if c.connQueue.IsEmpty() {
			break
		}
		fd := c.connQueue.Dequeue()

		err := c.handleNewConn(fd)
		if err != nil {
			c.logError(err).Msg("add request error")
		}
	}
}

func (c *consumerWorker) handleJobsInQueues() {
	if c.connQueue != nil {
		c.getConnsFromQueue()
	}

	c.handleAsyncWritesIfEnabled()
}

func (c *consumerWorker) loop(_ int) error {
	c.logInfo().Msg("Starting consumer loop...")
	c.prepareHandler = func() error {
		c.startedChan <- done

		return nil
	}
	c.shutdownHandler = func() bool {
		if c.needToShutdown() {
			c.onCloseHandler()
			c.markShutdownInProgress()
		}

		return true
	}
	c.loopFinisher = c.handleJobsInQueues
	c.loopFinishCondition = func() bool {
		if c.connectionManager.allClosed() {
			c.close()
			c.notifyFinish()

			return true
		}

		return false
	}

	return c.looper.startLoop(c.index(), func(cqe *giouring.CompletionQueueEvent) error {
		if exit := c.processEvent(cqe, func(cqe *giouring.CompletionQueueEvent) bool {
			keyOrFd := cqe.UserData & ^allFlagsMask

			return c.connectionManager.get(int(keyOrFd), 0) == nil
		}); exit {
			return nil
		}
		if cqe.UserData&addConnFlag > 0 {
			fd := int(cqe.Res)

			return c.handleNewConn(fd)
		}
		fileDescriptor := int(cqe.UserData & ^allFlagsMask)
		if fileDescriptor < syscall.Stderr {
			c.logError(nil).Int("fd", fileDescriptor).Msg("Invalid file descriptor")

			return nil
		}
		conn := c.connectionManager.getFd(fileDescriptor)
		c.handleConn(conn, cqe)

		return nil
	})
}

func newConsumerWorker(
	index int, localAddr net.Addr, config consumerConfig, eventHandler EventHandler, features supportedFeatures,
) (*consumerWorker, error) {
	ring, err := giouring.CreateRing(uint32(config.maxSQEntries))
	if err != nil {
		return nil, fmt.Errorf("creating ring error: %w", err)
	}
	logger := logger.NewLogger("consumer", config.loggerLevel, config.prettyLogger)
	connectionManager := newConnectionManager()
	consumer := &consumerWorker{
		config: config,
		readWriteWorkerImpl: newReadWriteWorkerImpl(
			ring, index, localAddr, eventHandler, connectionManager, config.readWriteWorkerConfig, logger,
		),
	}

	if !features.ringsMessaging {
		consumer.connQueue = queue.NewIntQueue()
	}
	consumer.onCloseHandler = consumer.closeAllConns

	return consumer, nil
}
