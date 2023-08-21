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
	"syscall"

	"github.com/pawelgaczynski/giouring"
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
	close()
}

type workerStartListener func()

type workerConfig struct {
	cpuAffinity     bool
	processPriority bool
	maxSQEntries    uint
	maxCQEvents     int
	loggerLevel     zerolog.Level
	prettyLogger    bool
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
	return w.looper.ring.RingFd()
}

func (w *workerImpl) setIndex(index int) {
	w.idx = index
}

func (w *workerImpl) index() int {
	return w.idx
}

func (w *workerImpl) processEvent(cqe *giouring.CompletionQueueEvent,
	skipErrorChecker func(*giouring.CompletionQueueEvent) bool,
) bool {
	w.logDebug().
		Int32("Res", cqe.Res).
		Uint64("req key", cqe.UserData & ^allFlagsMask).
		Uint64("user data", cqe.UserData).
		Str("req flag", flagToString(cqe.UserData)).
		Msg("Process event")

	switch {
	case cqe.Res < 0:
		if !skipErrorChecker(cqe) {
			w.logError(nil).
				Str("error", unix.ErrnoName(-syscall.Errno(cqe.Res))).
				Str("req flag", flagToString(cqe.UserData)).
				Uint64("req key", cqe.UserData & ^allFlagsMask).
				Uint64("user data", cqe.UserData).
				Msg("worker request returns error code")
		}

		return true

	case cqe.UserData == 0:
		w.logError(nil).
			Msg("user data flag is missing")

		return true
	}

	return false
}

func (w *workerImpl) logDebug() *zerolog.Event {
	return w.logger.Debug().Int("worker index", w.index()).Int("ring fd", w.ringFd())
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

func (w *workerImpl) close() {
	if w.looper.ring != nil {
		w.looper.ring.QueueExit()
	}
}

func newWorkerImpl(
	ring *giouring.Ring, config workerConfig, index int, logger zerolog.Logger,
) *workerImpl {
	return &workerImpl{
		logger:      logger,
		shutdowner:  newShutdowner(),
		connCloser:  newConnCloser(ring, logger),
		looper:      newLooper(ring, config.cpuAffinity, config.processPriority, config.maxCQEvents),
		startedChan: make(chan startSignal),
		idx:         index,
	}
}
