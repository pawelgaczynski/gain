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

package iouring

import (
	"fmt"
	"math"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"unsafe"
)

const (
	CQEFBuffer uint32 = 1 << iota
	CQEFMore
	CQEFSockNonempty
	CQEFNotif
)
const CQEBufferShift uint32 = 16

const CQEventFdDisabled uint32 = 1 << 0

type CompletionQueueEvent struct {
	userData uint64
	res      int32
	flags    uint32
}

func (c *CompletionQueueEvent) UserData() uint64 {
	return c.userData
}

func (c *CompletionQueueEvent) Res() int32 {
	return c.res
}

func (c *CompletionQueueEvent) Flags() uint32 {
	return c.flags
}

func (c *CompletionQueueEvent) FlagsString() string {
	flagsStrings := make([]string, 0)
	if c.flags&CQEFBuffer > 0 {
		flagsStrings = append(flagsStrings, "CQEFBuffer")
	}

	if c.flags&CQEFMore > 0 {
		flagsStrings = append(flagsStrings, "CQEFMore")
	}

	if c.flags&CQEFSockNonempty > 0 {
		flagsStrings = append(flagsStrings, "CQEFSockNonempty")
	}

	if c.flags&CQEFNotif > 0 {
		flagsStrings = append(flagsStrings, "CQEFNotif")
	}

	return strings.Join(flagsStrings, " | ")
}

type getData struct {
	submit, waitNr uint32
	flags          uint32
	arg            unsafe.Pointer
	sz             int
}

type getEventsArg struct {
	sigMask   uintptr
	sigMaskSz uint32
	pad       uint32
	ts        uintptr
}

func (ring *Ring) peekBatchCQEInternal(cqes []*CompletionQueueEvent) int {
	ready := atomic.LoadUint32(ring.cqRing.tail) - atomic.LoadUint32(ring.cqRing.head)
	count := min(len(cqes), int(ready))

	if ready != 0 {
		head := atomic.LoadUint32(ring.cqRing.head)
		mask := atomic.LoadUint32(ring.cqRing.ringMask)
		last := head + uint32(count)

		for i := 0; head != last; head, i = head+1, i+1 {
			cqes[i] = (*CompletionQueueEvent)(
				unsafe.Add(
					unsafe.Pointer(ring.cqRing.cqeBuff),
					uintptr(head&mask)*unsafe.Sizeof(CompletionQueueEvent{}),
				),
			)
		}
	}

	return count
}

func (ring *Ring) PeekBatchCQE(cqes []*CompletionQueueEvent) int {
	numberOfCQEs := ring.peekBatchCQEInternal(cqes)
	if numberOfCQEs == 0 {
		if ring.cqRingNeedsFlush() {
			flags := EnterGetEvents
			if ring.intFlags&IntFlagRegRing > 0 {
				flags |= EnterRegisteredRing
			}

			_, _ = ring.enter(0, 0, flags, nil)
			numberOfCQEs = ring.peekBatchCQEInternal(cqes)
		}
	}

	return numberOfCQEs
}

func (ring *Ring) getCQEInternal(getData *getData, looped bool) (*CompletionQueueEvent, uint32, error) {
	var (
		err       error
		event     *CompletionQueueEvent
		needEnter bool
		flags     uint32
		available uint32
	)

	available, event, err = ring.peekCQE()
	if err != nil {
		return event, flags, err
	}

	if event == nil && getData.waitNr == 0 && getData.submit == 0 {
		if looped || !ring.cqRingNeedsEnter() {
			err = ErrAgain

			return event, flags, err
		}

		needEnter = true
	}

	if getData.waitNr > available || needEnter {
		flags = EnterGetEvents | getData.flags
		needEnter = true
	}

	if getData.submit != 0 && ring.sqRingNeedsEnter(&flags) {
		needEnter = true
	}

	if !needEnter {
		return event, flags, err
	}

	if ring.intFlags&IntFlagRegRing > 0 {
		flags |= EnterRegisteredRing
	}

	return event, flags, err
}

func (ring *Ring) getCQEAndEnter(getData *getData) (*CompletionQueueEvent, error) {
	var (
		err    error
		event  *CompletionQueueEvent
		looped bool
		flags  uint32
	)

	for {
		event, flags, err = ring.getCQEInternal(getData, looped)
		if err != nil {
			break
		}

		var consumed uint

		consumed, err = ring.enter2(getData.submit, getData.waitNr, flags, getData.arg, getData.sz)
		if err != nil {
			break
		}

		getData.submit -= uint32(consumed)

		if event != nil {
			break
		}

		looped = true
	}

	return event, err
}

func (ring *Ring) getCQE(submitted, waitNr uint32) (*CompletionQueueEvent, error) {
	getData := &getData{
		submit: submitted,
		waitNr: waitNr,
		flags:  0,
		sz:     nSig / szDivider,
		arg:    unsafe.Pointer(nil),
	}
	cqe, err := ring.getCQEAndEnter(getData)
	runtime.KeepAlive(getData)

	return cqe, err
}

func (ring *Ring) WaitCQENr(waitNr uint32) (*CompletionQueueEvent, error) {
	return ring.getCQE(0, waitNr)
}

func (ring *Ring) WaitCQE() (*CompletionQueueEvent, error) {
	return ring.WaitCQENr(1)
}

func (ring *Ring) CQESeen(event *CompletionQueueEvent) {
	if event != nil {
		ring.CQAdvance(1)
	}
}

func (ring *Ring) cqRingNeedsFlush() bool {
	return atomic.LoadUint32(ring.sqRing.flags)&SQCQOverflow != 0
}

func (ring *Ring) cqRingNeedsEnter() bool {
	return (ring.flags&SetupIOPoll != 0) || ring.cqRingNeedsFlush()
}

func (ring *Ring) CQAdvance(numberOfCQEs uint32) {
	atomic.StoreUint32(ring.cqRing.head, *ring.cqRing.head+numberOfCQEs)
}

func (ring *Ring) peekCQE() (uint32, *CompletionQueueEvent, error) {
	var (
		err       error
		event     *CompletionQueueEvent
		available uint32
		mask      = *ring.cqRing.ringMask
	)

	for {
		tail := atomic.LoadUint32(ring.cqRing.tail)
		head := atomic.LoadUint32(ring.cqRing.head)
		event = nil

		available = tail - head
		if available == 0 {
			break
		}
		event = (*CompletionQueueEvent)(
			unsafe.Add(unsafe.Pointer(ring.cqRing.cqeBuff), uintptr(head&mask)*unsafe.Sizeof(CompletionQueueEvent{})),
		)

		if !(ring.params.features&FeatExtArg != 0) && event.UserData() == math.MaxUint64 {
			if event.Res() < 0 {
				err = syscall.Errno(uintptr(-event.Res()))
			}

			ring.CQAdvance(1)

			if err == nil {
				continue
			}
			event = nil
		}

		break
	}

	if err != nil {
		err = fmt.Errorf("peekCQE error: %w", err)
	}

	return available, event, err
}
