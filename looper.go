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
	"errors"
	"runtime"

	"github.com/pawelgaczynski/gain/iouring"
	gainErrors "github.com/pawelgaczynski/gain/pkg/errors"
)

type eventProcessor func(*iouring.CompletionQueueEvent) error

type looper struct {
	submitter
	ring            *iouring.Ring
	cpuAffinity     bool
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
			if errors.Is(err, gainErrors.ErrSkippable) {
				if l.loopFinisher != nil {
					l.loopFinisher()
				}

				if l.loopFinishCondition != nil && l.loopFinishCondition() {
					return nil
				}

				continue
			}

			return err
		}
		numberOfCQEs := l.ring.PeekBatchCQE(cqes)

		for i := 0; i < numberOfCQEs; i++ {
			cqe := cqes[i]

			err = eventProcessor(cqe)
			if err != nil {
				l.advance(uint32(i + 1))

				return err
			}
		}
		l.advance(uint32(numberOfCQEs))

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
		err = setProcessPriority()
		if err != nil {
			return err
		}
	}

	if l.cpuAffinity {
		err = setCPUAffinity(index)
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
	ring *iouring.Ring, cpuAffinity bool, processPriority bool, maxCQEvents int,
) *looper {
	return &looper{
		submitter:       newBatchSubmitter(ring),
		ring:            ring,
		cpuAffinity:     cpuAffinity,
		processPriority: processPriority,
		maxCQEvents:     maxCQEvents,
	}
}
