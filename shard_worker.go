package gain

import (
	"fmt"
	"syscall"

	"github.com/pawelgaczynski/gain/iouring"
	"github.com/pawelgaczynski/gain/logger"
	"go.uber.org/multierr"
	"golang.org/x/sys/unix"
)

type shardWorkerConfig struct {
	readWriteWorkerConfig
}

type shardWorker struct {
	*acceptor
	*readWriteWorkerImpl
	ring           *iouring.Ring
	connectionPool *connectionPool
	lockOSThread   bool
	acceptStopped  bool
}

func (w *shardWorker) accept(cqe *iouring.CompletionQueueEvent) error {
	if cqe.Res() < 0 {
		return fmt.Errorf("accept request failed: %d for conn: %d", -cqe.Res(), cqe.UserData())
	}
	fileDescriptor := int(cqe.Res())
	conn, err := w.connectionPool.get(fileDescriptor)
	if err != nil {
		return fmt.Errorf("fd: %d, getting connection error: %w", fileDescriptor, err)
	}
	conn.fd = fileDescriptor
	if w.lockOSThread {
		err = unix.SetsockoptInt(fileDescriptor, unix.SOL_SOCKET, unix.SO_INCOMING_CPU, w.index()+1)
		if err != nil {
			return fmt.Errorf("fd: %d, setting SO_INCOMING_CPU error: %w", fileDescriptor, err)
		}
	}
	_, err = w.addReadRequest(conn)
	if err != nil {
		return fmt.Errorf("add read request error: %w", err)
	}
	w.eventHandler.OnOpen(conn.fd)
	_, err = w.addAcceptConnRequest()
	if err != nil {
		w.acceptStopped = true
		return fmt.Errorf("add accept() request error: %w", err)
	}
	return nil
}

func (w *shardWorker) stopAccept() error {
	return w.syscallShutdownSocket(w.socket)
}

func (w *shardWorker) activeConnections() int {
	return w.connectionPool.activeConnections(func(c *connection) bool {
		return c.fd != w.socket
	})
}

func (w *shardWorker) handleConn(conn *connection, cqe *iouring.CompletionQueueEvent) {
	var err error
	var errMsg string
	switch conn.state {
	case connAccept:
		err = w.accept(cqe)
		if err != nil {
			errMsg = "accept error"
		}
	case connRead:
		err = w.read(cqe, conn.fd)
		if err != nil {
			errMsg = "read error"
		}
	case connWrite:
		w.eventHandler.AfterWrite(conn)
		err = w.write(cqe, conn)
		if err != nil {
			errMsg = "write error"
		}
	case connClose:
		if cqe.UserData()&closeConnFlag > 0 {
			err := w.connectionPool.release(conn.fd)
			if err != nil {
				errMsg = "close error"
			}
		}
	default:
		err = fmt.Errorf("unknown connection state")
	}
	if err != nil {
		shutdownErr := w.syscallShutdownSocket(conn.fd)
		if shutdownErr != nil {
			err = multierr.Combine(err, shutdownErr)
		}
		w.logError(err).Msg(errMsg)
	}
}

func (w *shardWorker) loop(socket int) error {
	w.logInfo().Int("socket", socket).Msg("Starting worker loop...")
	w.socket = socket
	w.prepareHandler = func() error {
		w.startedChan <- done
		_, err := w.addAcceptConnRequest()
		if err != nil {
			w.acceptStopped = true
		}
		return err
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
		if w.connectionPool.allClosed() || (w.acceptStopped && w.activeConnections() == 0) {
			w.notifyFinish()
			return true
		}
		return false
	}

	loopErr := w.startLoop(w.index(), func(cqe *iouring.CompletionQueueEvent) error {
		var err error
		if exit := w.processEvent(cqe); exit {
			return nil
		}
		fileDescriptor := int(cqe.UserData() & ^allFlagsMask)
		if fileDescriptor < syscall.Stderr {
			w.logError(nil).Int("fd", fileDescriptor).Msg("Invalid file descriptor")
			return nil
		}
		conn, err := w.connectionPool.get(fileDescriptor)
		if err != nil {
			shutdownErr := w.syscallShutdownSocket(fileDescriptor)
			if shutdownErr != nil {
				err = multierr.Combine(err, shutdownErr)
			}
			w.logError(err).Int("fd", fileDescriptor).Msg("Get connection error")
			return nil
		}
		w.handleConn(conn, cqe)
		return nil
	})
	err := w.stopAccept()
	if err != nil {
		w.logError(err).Msg("Socket shutdown error")
	}
	return loopErr
}

func (w *shardWorker) closeConnsAndRings() {
	w.logWarn().Msg("Closing connections")
	err := w.syscallShutdownSocket(w.socket)
	if err != nil {
		w.logError(err).Int("socket", w.socket).Msg("Socket shutdown error")
	}
	err = w.connectionPool.close(func(conn *connection) error {
		_, err := w.addCloseConnRequest(conn)
		if err != nil {
			w.logError(err).Msg("Add close() connection request error")
		}
		return err
	}, w.socket)
	if err != nil {
		w.logError(err).Msg("Closing connections error")
	}
}

func newShardWorker(index int, config shardWorkerConfig, eventHandler EventHandler) (*shardWorker, error) {
	ring, err := iouring.CreateRing()
	if err != nil {
		return nil, err
	}
	logger := logger.NewLogger("worker", config.loggerLevel, config.prettyLogger)
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
	worker := &shardWorker{
		acceptor:            newAcceptor(ring, connectionPool),
		readWriteWorkerImpl: readWriteWorkerImpl,
		ring:                ring,
		connectionPool:      connectionPool,
		lockOSThread:        config.lockOSThread,
	}
	worker.onCloseHandler = worker.closeConnsAndRings
	return worker, nil
}
