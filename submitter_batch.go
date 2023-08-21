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
	"fmt"
	"syscall"
	"time"

	gainErrors "github.com/pawelgaczynski/gain/pkg/errors"
	"github.com/pawelgaczynski/giouring"
)

var waitForArray = []uint32{
	1,
	32,
	64,
	96,
	128,
	256,
	384,
	512,
	768,
	1024,
	1536,
	2048,
	3072,
	4096,
	5120,
	6144,
	7168,
	8192,
	10240,
}

type batchSubmitter struct {
	ring            *giouring.Ring
	timeoutTimeSpec syscall.Timespec
	waitForIndex    uint32
	waitFor         uint32
}

func (s *batchSubmitter) submit() error {
	_, err := s.ring.SubmitAndWaitTimeout(s.waitFor, &s.timeoutTimeSpec, nil)
	if errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EINTR) ||
		errors.Is(err, syscall.ETIME) {
		if s.waitForIndex != 0 {
			s.waitForIndex--
			s.waitFor = waitForArray[s.waitForIndex]
		}

		return gainErrors.ErrSkippable
	}

	if err != nil {
		return fmt.Errorf("submitAndWaitTimeout error: %w", err)
	}

	return nil
}

func (s *batchSubmitter) advance(n uint32) {
	s.ring.CQAdvance(n)

	var (
		index           uint32
		lenWaitForArray = uint32(len(waitForArray))
	)

	for index = 1; index < lenWaitForArray; index++ {
		if waitForArray[index] > n {
			break
		}
		s.waitForIndex = index
	}
	s.waitFor = waitForArray[s.waitForIndex]
}

func newBatchSubmitter(ring *giouring.Ring) *batchSubmitter {
	submitter := &batchSubmitter{
		ring:            ring,
		timeoutTimeSpec: syscall.NsecToTimespec((time.Millisecond).Nanoseconds()),
	}
	submitter.waitFor = waitForArray[submitter.waitForIndex]

	return submitter
}
