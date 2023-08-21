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
	"io"
	"net"

	"github.com/alitto/pond"
	"github.com/pawelgaczynski/gain/pkg/queue"
	"github.com/pawelgaczynski/giouring"
	"github.com/rs/zerolog"
)

type readWriteWorker interface {
	worker
	activeConnections() int
}

type readWriteWorkerConfig struct {
	workerConfig
	asyncHandler  bool
	goroutinePool bool
	sendRecvMsg   bool
}

type readWriteWorkerImpl struct {
	*workerImpl
	*writer
	*reader
	ring              *giouring.Ring
	connectionManager *connectionManager
	asyncOpQueue      queue.LockFreeQueue[*connection]
	pool              *pond.WorkerPool
	eventHandler      EventHandler
	asyncHandler      bool
	goroutinePool     bool
	sendRecvMsg       bool
	localAddr         net.Addr
}

func (w *readWriteWorkerImpl) handleAsyncWritesIfEnabled() {
	if w.asyncHandler {
		w.handleAsyncWrites()
	}
}

func (w *readWriteWorkerImpl) handleAsyncWrites() {
	for {
		if w.asyncOpQueue.IsEmpty() {
			break
		}
		conn := w.asyncOpQueue.Dequeue()

		var err error

		switch conn.nextAsyncOp {
		case readOp:
			err = w.addReadRequest(conn)
			if err != nil {
				w.logError(err).Int("fd", conn.fd)

				continue
			}

		case writeOp:
			closed := conn.isClosed()

			if w.sendRecvMsg {
				conn.setMsgHeaderWrite()
			}

			err = w.addWriteRequest(conn, closed)
			if err != nil {
				w.logError(err).Int("fd", conn.fd)

				continue
			}

			if closed {
				err = w.addCloseConnRequest(conn)
				if err != nil {
					w.logError(err).Int("fd", conn.fd)

					continue
				}
			}

		case closeOp:
			err = w.addCloseConnRequest(conn)
			if err != nil {
				w.logError(err).Int("fd", conn.fd)

				continue
			}
		}
	}
}

func (w *readWriteWorkerImpl) work(conn *connection, n int) {
	conn.setUserSpace()
	w.eventHandler.OnRead(conn, n)
}

func (w *readWriteWorkerImpl) doAsyncWork(conn *connection, n int) func() {
	return func() {
		w.work(conn, n)

		switch {
		case conn.OutboundBuffered() > 0:
			conn.nextAsyncOp = writeOp
		case conn.isClosed():
			conn.nextAsyncOp = closeOp
		default:
			conn.nextAsyncOp = readOp
		}

		w.asyncOpQueue.Enqueue(conn)
	}
}

func (w *readWriteWorkerImpl) writeData(conn *connection) error {
	if w.sendRecvMsg {
		conn.setMsgHeaderWrite()
	}
	closed := conn.isClosed()

	err := w.addWriteRequest(conn, closed)
	if err != nil {
		return err
	}

	if closed {
		return w.addCloseConnRequest(conn)
	}

	return nil
}

func (w *readWriteWorkerImpl) onRead(cqe *giouring.CompletionQueueEvent, conn *connection) error {
	// https://manpages.debian.org/unstable/manpages-dev/recv.2.en.html
	// These calls return the number of bytes received, or -1 if an error occurred.
	// In the event of an error, errno is set to indicate the error.
	// When a stream socket peer has performed an orderly shutdown,
	// the return value will be 0 (the traditional "end-of-file" return).
	// Datagram sockets in various domains (e.g., the UNIX and Internet domains) permit zero-length datagrams.
	// When such a datagram is received, the return value is 0.
	// The value 0 may also be returned if the requested number of bytes to receive from a stream socket was 0.
	if cqe.Res <= 0 {
		w.closeConn(conn, true, io.EOF)

		return nil
	}

	w.logDebug().Int("fd", conn.fd).Int32("count", cqe.Res).Msg("Bytes read")

	n := int(cqe.Res)
	conn.onKernelRead(n)

	if w.sendRecvMsg {
		forkedConn := w.connectionManager.fork(conn, true)
		forkedConn.localAddr = w.localAddr

		err := w.addReadRequest(conn)
		if err != nil {
			return err
		}

		conn = forkedConn
	}

	if cqe.Flags&giouring.CQEFSockNonempty > 0 && !conn.isClosed() {
		return w.addReadRequest(conn)
	}

	if w.asyncHandler {
		if w.goroutinePool {
			w.pool.Submit(w.doAsyncWork(conn, n))
		} else {
			go w.doAsyncWork(conn, n)()
		}
	} else {
		w.work(conn, n)

		switch {
		case conn.OutboundBuffered() > 0:
			return w.writeData(conn)
		case conn.isClosed():
			err := w.addCloseConnRequest(conn)
			if err != nil {
				return err
			}
		default:
			err := w.addReadRequest(conn)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (w *readWriteWorkerImpl) addNextRequest(conn *connection) error {
	closed := conn.isClosed()

	switch {
	case conn.OutboundBuffered() > 0:
		err := w.addWriteRequest(conn, closed)
		if err != nil {
			return fmt.Errorf("add read() request error: %w", err)
		}

		if closed {
			err = w.addCloseConnRequest(conn)
			if err != nil {
				return fmt.Errorf("add close() request error: %w", err)
			}
		}

	case closed:
		err := w.addCloseConnRequest(conn)
		if err != nil {
			return fmt.Errorf("add close() request error: %w", err)
		}

	default:
		err := w.addReadRequest(conn)
		if err != nil {
			return fmt.Errorf("add read() request error: %w", err)
		}
	}

	return nil
}

func (w *readWriteWorkerImpl) closeConn(conn *connection, syscallClose bool, err error) {
	if syscallClose {
		_ = w.syscallCloseSocket(conn.fd)
	}

	conn.setUserSpace()

	if !conn.isClosed() {
		conn.Close()
	}

	w.eventHandler.OnClose(conn, err)
	w.connectionManager.release(conn.key)
}

func newReadWriteWorkerImpl(ring *giouring.Ring, index int, localAddr net.Addr, eventHandler EventHandler,
	connectionManager *connectionManager, config readWriteWorkerConfig, logger zerolog.Logger,
) *readWriteWorkerImpl {
	worker := &readWriteWorkerImpl{
		workerImpl:        newWorkerImpl(ring, config.workerConfig, index, logger),
		reader:            newReader(ring, config.sendRecvMsg),
		writer:            newWriter(ring, config.sendRecvMsg),
		ring:              ring,
		connectionManager: connectionManager,
		asyncOpQueue:      queue.NewQueue[*connection](),
		eventHandler:      eventHandler,
		asyncHandler:      config.asyncHandler,
		goroutinePool:     config.goroutinePool,
		sendRecvMsg:       config.sendRecvMsg,
		localAddr:         localAddr,
	}
	if config.asyncHandler && config.goroutinePool {
		worker.pool = pond.New(goPoolMaxWorkers, goPoolMaxCapacity)
	}

	return worker
}
