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
	gainErrors "github.com/pawelgaczynski/gain/pkg/errors"
	"github.com/pawelgaczynski/gain/pkg/socket"
	"golang.org/x/sys/unix"
)

type shardWorkerConfig struct {
	readWriteWorkerConfig
	tcpKeepAlive time.Duration
}

type shardWorker struct {
	*acceptor
	*readWriteWorkerImpl
	ring               *iouring.Ring
	connectionManager  *connectionManager
	cpuAffinity        bool
	tcpKeepAlive       time.Duration
	connectionProtocol bool
	accepting          atomic.Bool
}

func (w *shardWorker) onAccept(cqe *iouring.CompletionQueueEvent) error {
	err := w.addAcceptConnRequest()
	if err != nil {
		w.accepting.Store(false)

		return fmt.Errorf("add accept() request error: %w", err)
	}

	fileDescriptor := int(cqe.Res())
	w.logDebug().Int("fd", fileDescriptor).Msg("Connection accepted")

	conn := w.connectionManager.getFd(fileDescriptor)
	if conn == nil {
		return gainErrors.ErrorConnectionIsMissing(fileDescriptor)
	}
	conn.fd = fileDescriptor
	conn.localAddr = w.localAddr

	var clientAddr net.Addr
	if clientAddr, err = w.acceptor.lastClientAddr(); err == nil {
		conn.remoteAddr = clientAddr
	} else {
		w.logError(err).Msg("get last client address failed")
	}

	if w.cpuAffinity {
		err = unix.SetsockoptInt(fileDescriptor, unix.SOL_SOCKET, unix.SO_INCOMING_CPU, w.index()+1)
		if err != nil {
			return fmt.Errorf("fd: %d, setting SO_INCOMING_CPU error: %w", fileDescriptor, err)
		}
	}

	if w.tcpKeepAlive > 0 {
		err = socket.SetKeepAlivePeriod(fileDescriptor, int(w.tcpKeepAlive.Seconds()))
		if err != nil {
			return fmt.Errorf("fd: %d, setting tcpKeepAlive error: %w", fileDescriptor, err)
		}
	}

	conn.setUserSpace()
	w.eventHandler.OnAccept(conn)

	return w.addNextRequest(conn)
}

func (w *shardWorker) stopAccept() error {
	return w.syscallCloseSocket(w.fd)
}

func (w *shardWorker) activeConnections() int {
	return w.connectionManager.activeConnections(func(c *connection) bool {
		return c.fd != w.fd
	})
}

func (w *shardWorker) handleConn(conn *connection, cqe *iouring.CompletionQueueEvent) {
	var (
		err    error
		errMsg string
	)

	switch conn.state {
	case connAccept:
		err = w.onAccept(cqe)
		if err != nil {
			errMsg = "accept error"
		}

	case connRead:
		err = w.onRead(cqe, conn)
		if err != nil {
			errMsg = "read error"
		}

	case connWrite:
		if !w.connectionProtocol && conn.key == 1 {
			errMsg = "main socket cannot be in write mode"

			break
		}

		n := int(cqe.Res())
		conn.onKernelWrite(n)
		w.logDebug().Int("fd", conn.fd).Int32("count", cqe.Res()).Msg("Bytes writed")

		conn.setUserSpace()
		w.eventHandler.OnWrite(conn, n)

		if w.sendRecvMsg {
			w.connectionManager.release(conn.key)

			break
		}

		err = w.addNextRequest(conn)
		if err != nil {
			errMsg = "add request error"
		}

	case connClose:
		if cqe.UserData()&closeConnFlag > 0 {
			w.closeConn(conn, false, nil)
		} else if cqe.UserData()&writeDataFlag > 0 {
			n := int(cqe.Res())
			conn.onKernelWrite(n)
			w.logDebug().Int("fd", conn.fd).Int32("count", cqe.Res()).Msg("Bytes writed")
			conn.setUserSpace()
			w.eventHandler.OnWrite(conn, n)
			// conn.setUserSpace()
			// w.eventHandler.OnWrite(conn)
		}

	default:
		err = gainErrors.ErrorUnknownConnectionState(int(conn.state))
	}

	if err != nil {
		w.logError(err).Msg(errMsg)

		w.closeConn(conn, true, err)
	}
}

func (w *shardWorker) initLoop() {
	if w.connectionProtocol {
		w.prepareHandler = func() error {
			w.startedChan <- done

			err := w.addAcceptConnRequest()
			if err == nil {
				w.accepting.Store(true)
			}

			return err
		}
	} else {
		w.prepareHandler = func() error {
			w.startedChan <- done
			// 1 is always index for main socket
			conn := w.connectionManager.get(1, w.fd)
			if conn == nil {
				return gainErrors.ErrorConnectionIsMissing(1)
			}
			conn.fd = w.fd
			conn.initMsgHeader()

			return w.addReadRequest(conn)
		}
	}
	w.shutdownHandler = func() bool {
		if w.needToShutdown() {
			w.onCloseHandler()
			w.markShutdownInProgress()
		}

		return true
	}
	w.loopFinisher = w.handleAsyncWritesIfEnabled
	w.loopFinishCondition = func() bool {
		if w.connectionManager.allClosed() || (w.connectionProtocol && !w.accepting.Load() && w.activeConnections() == 0) {
			w.notifyFinish()

			return true
		}

		return false
	}
}

func (w *shardWorker) loop(fd int) error {
	w.logInfo().Int("fd", fd).Msg("Starting worker loop...")
	w.fd = fd
	w.initLoop()

	loopErr := w.startLoop(w.index(), func(cqe *iouring.CompletionQueueEvent) error {
		fmt.Printf("%+v\n", cqe)
		var err error
		if exit := w.processEvent(cqe, func(cqe *iouring.CompletionQueueEvent) bool {
			keyOrFd := cqe.UserData() & ^allFlagsMask
			if acceptReqFailedAfterStop := keyOrFd == uint64(w.fd) &&
				!w.accepting.Load(); acceptReqFailedAfterStop {
				return true
			}

			return false
		}); exit {
			return nil
		}
		key := int(cqe.UserData() & ^allFlagsMask)
		var connFd int
		if w.connectionProtocol {
			connFd = key
		} else {
			connFd = w.fd
		}
		conn := w.connectionManager.get(key, connFd)
		if conn == nil {
			if w.connectionProtocol {
				_ = w.syscallCloseSocket(connFd)
			}
			logEvent := w.logError(err)
			if w.connectionProtocol {
				logEvent = logEvent.Int("conn fd", connFd)
			}

			logEvent.Msg("Get connection error")

			return nil
		}
		w.handleConn(conn, cqe)

		return nil
	})
	_ = w.stopAccept()

	return loopErr
}

func (w *shardWorker) closeAllConnsAndRings() {
	w.logWarn().Msg("Closing connections")
	w.accepting.Store(false)
	_ = w.syscallCloseSocket(w.fd)
	w.connectionManager.close(func(conn *connection) bool {
		err := w.addCloseConnRequest(conn)
		if err != nil {
			w.logError(err).Msg("Add close() connection request error")
		}

		return err == nil
	}, w.fd)
}

func newShardWorker(
	index int, localAddr net.Addr, config shardWorkerConfig, eventHandler EventHandler,
) (*shardWorker, error) {
	ring, err := iouring.CreateRing()
	if err != nil {
		return nil, fmt.Errorf("creating ring error: %w", err)
	}
	logger := logger.NewLogger("worker", config.loggerLevel, config.prettyLogger)
	connectionManager := newConnectionManager()
	connectionProtocol := !config.sendRecvMsg
	worker := &shardWorker{
		readWriteWorkerImpl: newReadWriteWorkerImpl(
			ring, index, localAddr, eventHandler, connectionManager, config.readWriteWorkerConfig, logger,
		),
		ring:               ring,
		connectionManager:  connectionManager,
		cpuAffinity:        config.cpuAffinity,
		tcpKeepAlive:       config.tcpKeepAlive,
		connectionProtocol: connectionProtocol,
		acceptor:           newAcceptor(ring, connectionManager),
	}
	worker.onCloseHandler = worker.closeAllConnsAndRings

	return worker, nil
}
