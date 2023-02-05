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
	"syscall"
	"time"

	"github.com/pawelgaczynski/gain/iouring"
)

type singleSubmitter struct {
	ring            *iouring.Ring
	timeoutTimeSpec syscall.Timespec
}

func (s *singleSubmitter) submit() error {
	_, err := s.ring.SubmitAndWaitTimeout(1, &s.timeoutTimeSpec)
	if errors.Is(err, iouring.ErrAgain) || errors.Is(err, iouring.ErrInterrupredSyscall) ||
		errors.Is(err, iouring.ErrTimerExpired) {
		return errSkippable
	}
	return err
}

func (s *singleSubmitter) advance(n uint32) {
	s.ring.CQAdvance(n)
}

func newSingleSubmitter(ring *iouring.Ring) *singleSubmitter {
	return &singleSubmitter{
		ring:            ring,
		timeoutTimeSpec: syscall.NsecToTimespec((time.Millisecond).Nanoseconds()),
	}
}
