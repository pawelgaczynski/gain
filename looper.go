package gain

import (
	"errors"
	"runtime"

	"github.com/pawelgaczynski/gain/iouring"
)

type eventProcessor func(*iouring.CompletionQueueEvent) error

type looper struct {
	submitter
	ring            *iouring.Ring
	lockOSThread    bool
	processPriority bool
	maxCQEvents     int
	startListener   workerStartListener

	prepareHandler      func() error
	loopFinisher        func()
	loopFinishCondition func() bool
	shutdownHandler     func() bool
	running             bool
}

func (l *looper) innerLoop(eventProcessor eventProcessor) error {
	var err error
	cqes := make([]*iouring.CompletionQueueEvent, l.maxCQEvents)
	for {
		if continueLoop := l.shutdownHandler(); !continueLoop {
			return nil
		}
		if err = l.submit(); err != nil {
			if errors.Is(err, errSkippable) {
				continue
			}
			return err
		}
		n := l.ring.PeekBatchCQE(cqes)
		for i := 0; i < n; i++ {
			cqe := cqes[i]
			err := eventProcessor(cqe)
			if err != nil {
				l.advance(uint32(i + 1))
				return err
			}
		}
		l.advance(uint32(n))
		if l.loopFinisher != nil {
			l.loopFinisher()
		}
		if l.loopFinishCondition != nil && l.loopFinishCondition() {
			return nil
		}
	}
}

func (l *looper) startLoop(index int, eventProcessor eventProcessor) error {
	var err error
	if l.processPriority {
		err := setProcessPriority()
		if err != nil {
			return err
		}
	}
	if l.lockOSThread {
		err := setAffinity(index)
		if err != nil {
			return err
		}
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
	}
	if l.prepareHandler != nil {
		err = l.prepareHandler()
		if err != nil {
			return err
		}
	}

	if l.startListener != nil {
		l.running = true
		l.startListener()
	}
	return l.innerLoop(eventProcessor)
}

func (l *looper) started() bool {
	return l.running
}

func newLooper(
	ring *iouring.Ring, lockOSThread bool, processPriority bool, maxCQEvents int, batchSubmitter bool) *looper {
	var submitter submitter
	if batchSubmitter {
		submitter = newBatchSubmitter(ring)
	} else {
		submitter = newSingleSubmitter(ring)
	}
	return &looper{
		submitter:       submitter,
		ring:            ring,
		lockOSThread:    lockOSThread,
		processPriority: processPriority,
		maxCQEvents:     maxCQEvents,
	}
}
