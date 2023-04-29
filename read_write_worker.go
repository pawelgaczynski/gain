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
	"net"

	"github.com/alitto/pond"
	"github.com/pawelgaczynski/gain/iouring"
	"github.com/pawelgaczynski/gain/pkg/queue"
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
	ring              *iouring.Ring
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
		}
	}
}

func (w *readWriteWorkerImpl) work(conn *connection) {
	conn.setUserSpace()
	w.eventHandler.OnRead(conn)
}

func (w *readWriteWorkerImpl) doAsyncWork(conn *connection) func() {
	return func() {
		w.work(conn)

		if conn.OutboundBuffered() > 0 {
			conn.nextAsyncOp = writeOp
		} else if !conn.isClosed() {
			conn.nextAsyncOp = readOp
		}

		w.asyncOpQueue.Enqueue(conn)
	}
}

func (w *readWriteWorkerImpl) onWrite(cqe *iouring.CompletionQueueEvent, conn *connection) error {
	conn.onKernelWrite(int(cqe.Res()))
	w.logDebug().Int("fd", conn.fd).Int32("count", cqe.Res()).Msg("Bytes writed")

	if w.sendRecvMsg {
		w.eventHandler.OnClose(conn)
		w.connectionManager.release(conn.key)

		return nil
	}

	return w.addReadRequest(conn)
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
		err = w.addCloseConnRequest(conn)

		return err
	}

	return nil
}

func (w *readWriteWorkerImpl) onRead(cqe *iouring.CompletionQueueEvent, conn *connection) error {
	// https://manpages.debian.org/unstable/manpages-dev/recv.2.en.html
	// These calls return the number of bytes received, or -1 if an error occurred.
	// In the event of an error, errno is set to indicate the error.
	// When a stream socket peer has performed an orderly shutdown,
	// the return value will be 0 (the traditional "end-of-file" return).
	// Datagram sockets in various domains (e.g., the UNIX and Internet domains) permit zero-length datagrams.
	// When such a datagram is received, the return value is 0.
	// The value 0 may also be returned if the requested number of bytes to receive from a stream socket was 0.
	if cqe.Res() <= 0 {
		_ = w.syscallCloseSocket(conn.fd)
		w.eventHandler.OnClose(conn)
		w.connectionManager.release(conn.key)

		return nil
	}

	w.logDebug().Int("fd", conn.fd).Int32("count", cqe.Res()).Msg("Bytes read")

	conn.onKernelRead(int(cqe.Res()))

	if w.sendRecvMsg {
		forkedConn := w.connectionManager.fork(conn, true)
		forkedConn.localAddr = w.localAddr

		err := w.addReadRequest(conn)
		if err != nil {
			return err
		}

		conn = forkedConn
	}

	if cqe.Flags()&iouring.CQEFSockNonempty > 0 && !conn.isClosed() {
		return w.addReadRequest(conn)
	}

	if w.asyncHandler {
		if w.goroutinePool {
			w.pool.Submit(w.doAsyncWork(conn))
		} else {
			go w.doAsyncWork(conn)()
		}
	} else {
		w.work(conn)
		if conn.OutboundBuffered() > 0 {
			return w.writeData(conn)
		} else if !conn.isClosed() {
			err := w.addReadRequest(conn)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func newReadWriteWorkerImpl(ring *iouring.Ring, index int, localAddr net.Addr, eventHandler EventHandler,
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
