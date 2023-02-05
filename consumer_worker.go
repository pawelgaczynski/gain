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
	"syscall"

	"github.com/pawelgaczynski/gain/iouring"
	"github.com/pawelgaczynski/gain/logger"
	"go.uber.org/multierr"
)

type consumerConfig struct {
	readWriteWorkerConfig
}

type consumer interface {
	readWriteWorker
	addConnToQueue(fd int) error
}

type consumerWorker struct {
	*readWriteWorkerImpl
	config consumerConfig

	// used for kernels < 5.18 where OP_MSG_RING is not supported
	connQueue lockFreeQueue[int]
}

func (c *consumerWorker) addConnToQueue(fd int) error {
	if c.connQueue == nil {
		return fmt.Errorf("Conn queue is nil")
	}
	c.connQueue.enqueue(fd)
	return nil
}

func (c *consumerWorker) closeConns() {
	c.logWarn().Msg("Closing connections")
	err := c.connectionPool.close(func(conn *connection) error {
		_, err := c.addCloseConnRequest(conn)
		if err != nil {
			c.logError(err).Msg("Add close() connection request error")
		}
		return err
	}, -1)
	if err != nil {
		c.logError(err).Msg("Closing connections error")
	}
}

func (c *consumerWorker) activeConnections() int {
	return c.connectionPool.activeConnections(func(c *connection) bool {
		return true
	})
}

func (c *consumerWorker) handleConn(conn *connection, cqe *iouring.CompletionQueueEvent) {
	var err error
	var errMsg string
	switch conn.state {
	case connRead:
		err = c.read(cqe, conn.fd)
		if err != nil {
			errMsg = "read error"
		}
	case connWrite:
		c.eventHandler.AfterWrite(conn)
		err = c.write(cqe, conn)
		if err != nil {
			errMsg = "write error"
		}
	case connClose:
		if cqe.UserData()&closeConnFlag > 0 {
			err := c.connectionPool.release(conn.fd)
			if err != nil {
				errMsg = "close error"
			}
		}
	default:
		err = fmt.Errorf("unknown connection state")
	}
	if err != nil {
		shutdownErr := c.syscallShutdownSocket(conn.fd)
		if shutdownErr != nil {
			err = multierr.Combine(err, shutdownErr)
		}
		c.logError(err).Msg(errMsg)
	}
}

func (c *consumerWorker) getConnsFromQueue() {
	for {
		if c.connQueue.isEmpty() {
			break
		}
		fd := c.connQueue.dequeue()
		conn, err := c.connectionPool.get(fd)
		if err != nil {
			c.logError(err).Msg("Get new connection error")
			continue
		}
		conn.fd = fd
		_, err = c.addReadRequest(conn)
		if err != nil {
			c.logError(err).Msg("Add read() request for new connection error")
			continue
		}
		c.eventHandler.OnOpen(conn.fd)
	}
}

func (c *consumerWorker) handleJobsInQueues() {
	if c.connQueue != nil {
		c.getConnsFromQueue()
	}
	c.handleAsyncWritesIfEnabled()
}

func (c *consumerWorker) loop(socket int) error {
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
		if c.connectionPool.allClosed() {
			c.notifyFinish()
			return true
		}
		return false
	}
	return c.looper.startLoop(c.index(), func(cqe *iouring.CompletionQueueEvent) error {
		if exit := c.processEvent(cqe); exit {
			return nil
		}
		if cqe.UserData()&addConnFlag > 0 {
			fileDescriptor := int(cqe.Res())
			conn, err := c.connectionPool.get(fileDescriptor)
			if err != nil {
				c.logError(err).Msg("Get new connection error")
				err := c.syscallShutdownSocket(fileDescriptor)
				if err != nil {
					c.logError(err).Int("conn fd", fileDescriptor).Msg("Can't shutdown socket")
				}
				return nil
			}
			conn.fd = int(cqe.Res())
			_, err = c.addReadRequest(conn)
			if err != nil {
				shutdownErr := c.syscallShutdownSocket(fileDescriptor)
				if shutdownErr != nil {
					err = multierr.Combine(err, shutdownErr)
				}
				c.logError(err).Msg("Add read() request for new connection error")
				return nil
			}
			c.eventHandler.OnOpen(conn.fd)
			return nil
		}
		fileDescriptor := int(cqe.UserData() & ^allFlagsMask)
		if fileDescriptor < syscall.Stderr {
			c.logError(nil).Int("fd", fileDescriptor).Msg("Invalid file descriptor")
			return nil
		}
		conn, err := c.connectionPool.get(fileDescriptor)
		if err != nil {
			shutdownErr := c.syscallShutdownSocket(fileDescriptor)
			if shutdownErr != nil {
				err = multierr.Combine(err, shutdownErr)
			}
			c.logError(err).Msg("Get connection error")
			return nil
		}
		c.handleConn(conn, cqe)
		return nil
	})
}

func newConsumerWorker(
	index int, config consumerConfig, eventHandler EventHandler, supportedFeatures SupportedFeatures,
) (*consumerWorker, error) {
	ring, err := iouring.CreateRing()
	if err != nil {
		return nil, err
	}
	logger := logger.NewLogger("consumer", config.loggerLevel, config.prettyLogger)
	connectionPool := newConnectionPool(
		config.maxConn,
		config.bufferSize,
		config.prefillConnectionPool,
	)
	readWriteWorkerImpl, err := newReadWriteWorkerImpl(
		ring, index, eventHandler, connectionPool, config.readWriteWorkerConfig, logger,
	)
	if err != nil {
		return nil, err
	}
	consumer := &consumerWorker{
		config:              config,
		readWriteWorkerImpl: readWriteWorkerImpl,
	}
	if !supportedFeatures.ringsMessaging {
		consumer.connQueue = newIntQueue()
	}
	consumer.onCloseHandler = consumer.closeConns
	return consumer, nil
}
