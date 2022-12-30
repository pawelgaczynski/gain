package gain

import (
	"fmt"

	"github.com/alitto/pond"
	"github.com/pawelgaczynski/gain/iouring"
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
	maxConn       uint
}

type readWriteWorkerImpl struct {
	*workerImpl
	*writer
	*reader
	ring           *iouring.Ring
	connectionPool *connectionPool
	writeQueue     lockFreeQueue[*connection]
	pool           *pond.WorkerPool
	eventHandler   EventHandler
	asyncHandler   bool
	goroutinePool  bool
}

func (w *readWriteWorkerImpl) handleAsyncWritesIfEnabled() {
	if w.asyncHandler {
		w.handleAsyncWrites()
	}
}

func (w *readWriteWorkerImpl) handleAsyncWrites() {
	for {
		if w.writeQueue.isEmpty() {
			break
		}
		conn := w.writeQueue.dequeue()
		_, err := w.addWriteRequest(conn)
		if err != nil {
			w.logError(err).Int("fd", conn.fd)
			continue
		}
	}
}

func (w *readWriteWorkerImpl) work(conn *connection, res int) error {
	conn.setReadOrWriteUser()
	err := w.eventHandler.OnData(conn)
	if err != nil {
		return err
	}
	if w.asyncHandler && conn.anyWrites {
		w.writeQueue.enqueue(conn)
	}
	return nil
}

func (w *readWriteWorkerImpl) doWork(conn *connection, cqe *iouring.CompletionQueueEvent) func() {
	return func() {
		err := w.work(conn, int(cqe.Res()))
		if err != nil {
			w.logError(err).Msg("Work error")
		}
	}
}

func (w *readWriteWorkerImpl) write(cqe *iouring.CompletionQueueEvent, conn *connection) error {
	if cqe.Res() < 0 {
		return fmt.Errorf("Write request failed: %d for conn: %d", -cqe.Res(), conn.fd)
	}
	_, err := w.addReadRequest(conn)
	if err != nil {
		return err
	}
	return nil
}

func (w *readWriteWorkerImpl) read(cqe *iouring.CompletionQueueEvent, fileDescriptor int) error {
	if cqe.Res() <= 0 {
		_ = w.syscallShutdownSocket(fileDescriptor)
		err := w.connectionPool.release(fileDescriptor)
		if err != nil {
			return err
		}
		return nil
	}

	conn, err := w.connectionPool.get(fileDescriptor)
	if err != nil {
		return err
	}
	conn.setSize(int(cqe.Res()))
	if w.asyncHandler {
		if w.goroutinePool {
			w.pool.Submit(w.doWork(conn, cqe))
		} else {
			go w.doWork(conn, cqe)()
		}
	} else {
		err = w.work(conn, int(cqe.Res()))
		if err != nil {
			return err
		}
		_, err = w.addWriteRequest(conn)
		if err != nil {
			return err
		}
	}
	return nil
}

func newReadWriteWorkerImpl(ring *iouring.Ring, index int, eventHandler EventHandler,
	connectionPool *connectionPool, config readWriteWorkerConfig, logger zerolog.Logger,
) (*readWriteWorkerImpl, error) {
	workerImpl, err := newWorkerImpl(ring, config.workerConfig, index, logger)
	if err != nil {
		return nil, err
	}
	worker := &readWriteWorkerImpl{
		workerImpl:     workerImpl,
		reader:         newReader(ring),
		writer:         newWriter(ring),
		ring:           ring,
		connectionPool: connectionPool,
		writeQueue:     newConnectionQueue(),
		eventHandler:   eventHandler,
		asyncHandler:   config.asyncHandler,
		goroutinePool:  config.goroutinePool,
	}
	if config.asyncHandler && config.goroutinePool {
		worker.pool = pond.New(goPoolMaxWorkers, goPoolMaxCapacity)
	}
	return worker, nil
}
