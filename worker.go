package gain

import (
	"syscall"

	"github.com/pawelgaczynski/gain/iouring"
	"github.com/rs/zerolog"
	"golang.org/x/sys/unix"
)

type startSignal int

const (
	done startSignal = iota
)

type worker interface {
	loop(socket int) error
	setIndex(index int)
	index() int
	shutdown()
	ringFd() int
	started() bool
}

type workerStartListener func()

type workerConfig struct {
	lockOSThread          bool
	processPriority       bool
	bufferSize            uint
	maxCQEvents           int
	batchSubmitter        bool
	prefillConnectionPool bool
	loggerLevel           zerolog.Level
	prettyLogger          bool
}

type workerImpl struct {
	*looper
	*connCloser
	*shutdowner
	idx            int
	logger         zerolog.Logger
	startedChan    chan startSignal
	onCloseHandler func()
}

func (w *workerImpl) ringFd() int {
	return w.looper.ring.Fd()
}

func (w *workerImpl) setIndex(index int) {
	w.idx = index
}

func (w *workerImpl) index() int {
	return w.idx
}

func (w *workerImpl) processEvent(cqe *iouring.CompletionQueueEvent) bool {
	switch {
	case cqe.Res() < 0:
		w.logError(nil).
			Str("error", unix.ErrnoName(-syscall.Errno(cqe.Res()))).
			Str("req flag", flagToString(cqe.UserData())).
			Uint64("req fd", cqe.UserData() & ^allFlagsMask).
			Uint64("user data", cqe.UserData()).
			Msg("worker request returns error code")
		return true
	case cqe.UserData() == 0:
		w.logError(nil).
			Msg("user data flag is missing")
		return true
	}
	return false
}

func (w *workerImpl) logInfo() *zerolog.Event {
	return w.logger.Info().Int("worker index", w.index()).Int("ring fd", w.ringFd())
}

func (w *workerImpl) logWarn() *zerolog.Event {
	return w.logger.Warn().Int("worker index", w.index()).Int("ring fd", w.ringFd())
}

func (w *workerImpl) logError(err error) *zerolog.Event {
	return w.logger.Error().Int("worker index", w.index()).Int("ring fd", w.ringFd()).Err(err)
}

func newWorkerImpl(
	ring *iouring.Ring, config workerConfig, index int, logger zerolog.Logger,
) (*workerImpl, error) {
	worker := &workerImpl{
		logger:      logger,
		shutdowner:  newShutdowner(),
		connCloser:  newConnCloser(ring, logger),
		looper:      newLooper(ring, config.lockOSThread, config.processPriority, config.maxCQEvents, config.batchSubmitter),
		startedChan: make(chan startSignal),
		idx:         index,
	}
	return worker, nil
}
