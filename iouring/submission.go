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
	"runtime"
	"sync/atomic"
	"syscall"
	"unsafe"
)

const FileIndexAlloc uint = 4294967295

const (
	SqeFixedFile uint8 = 1 << iota
	SqeIODrain
	SqeIOLink
	SqeIOHardlink
	SqeAsync
	SqeBufferSelect
	SqeCQESkipSuccess
)

const FsyncDatasync uint32 = 1 << 0

const (
	TimeoutAbs uint32 = 1 << iota
	TimeoutUpdate
	TimeoutBoottime
	TimeoutRealtime
	LinkTimeoutUpdate
	TimeoutETimeSuccess
	TimeoutClockMask  = TimeoutBoottime | TimeoutRealtime
	TimeoutUpdateMask = TimeoutUpdate | LinkTimeoutUpdate
)

const SpliceFFdInFixed uint32 = 1 << 31

const (
	PollAddMulti uint32 = 1 << iota
	PollUpdateEvents
	PollUpdateUserData
	PollAddLevel
)

const (
	AsyncCancelAll uint32 = 1 << iota
	AsyncCancelFd
	AsyncCancelAny
	AsyncCancelFdFixed
)

const (
	RecvsendPollFirst uint16 = 1 << iota
	RecvMultishot
	RecvsendFixedBuf
)

const (
	AcceptMultishot uint16 = 1 << iota
)

const (
	MsgData uint32 = iota
	MsgSendFd
)

const (
	MsgRingCQESkip uint32 = 1 << iota
)

type SubmissionQueueEntry struct {
	OpCode      uint8
	Flags       uint8
	IoPrio      uint16
	Fd          int32
	Off         uint64
	Addr        uint64
	Len         uint32
	OpcodeFlags uint32
	UserData    uint64

	BufIG       uint16
	Personality uint16
	SpliceFdIn  int32
	_pad2       [2]uint64
}

func (ring *Ring) GetSQE() (*SubmissionQueueEntry, error) {
	head := atomic.LoadUint32(ring.sqRing.head)
	next := ring.sqRing.sqeTail + 1

	var entry *SubmissionQueueEntry

	if next-head <= *ring.sqRing.ringEntries {
		idx := ring.sqRing.sqeTail & *ring.sqRing.ringMask * uint32(unsafe.Sizeof(SubmissionQueueEntry{}))
		entry = (*SubmissionQueueEntry)(unsafe.Pointer(&ring.sqRing.sqeBuffer[idx]))
		ring.sqRing.sqeTail = next
	} else {
		return nil, ErrorSQEOverflow(next - head)
	}

	return entry, nil
}

func (ring *Ring) FlushSQ() uint32 {
	mask := *ring.sqRing.ringMask
	tail := atomic.LoadUint32(ring.sqRing.tail)

	subCnt := ring.sqRing.sqeTail - ring.sqRing.sqeHead
	if subCnt == 0 {
		return tail - atomic.LoadUint32(ring.sqRing.head)
	}

	for i := subCnt; i > 0; i-- {
		*(*uint32)(
			unsafe.Add(unsafe.Pointer(ring.sqRing.array),
				tail&mask*uint32(unsafe.Sizeof(uint32(0))))) = ring.sqRing.sqeHead & mask
		tail++
		ring.sqRing.sqeHead++
	}
	atomic.StoreUint32(ring.sqRing.tail, tail)

	return tail - atomic.LoadUint32(ring.sqRing.head)
}

func (ring *Ring) sqRingNeedsEnter(flags *uint32) bool {
	if ring.flags&SetupSQPoll == 0 {
		return true
	}

	if atomic.LoadUint32(ring.sqRing.flags)&SQNeedWakeup > 0 {
		*flags |= EnterSQWakeup

		return true
	}

	return false
}

func (ring *Ring) SubmitInternal(submitted uint32, waitNr uint64) (uint, error) {
	var (
		flags uint32
		ret   uint
		err   error
	)

	if ring.sqRingNeedsEnter(&flags) || waitNr > 0 {
		if waitNr > 0 || (ring.flags&SetupIOPoll > 0) {
			flags |= EnterGetEvents
		}

		if ring.intFlags&IntFlagRegRing > 0 {
			flags |= EnterRegisteredRing
		}
		ret, err = ring.enter(submitted, uint32(waitNr), flags, nil)
	} else {
		ret = uint(submitted)
	}

	return ret, err
}

func (ring *Ring) SubmitAndWaitInternal(waitNr uint64) (uint, error) {
	return ring.SubmitInternal(ring.FlushSQ(), waitNr)
}

func (ring *Ring) SubmitAndWait(waitNr uint64) (uint, error) {
	return ring.SubmitAndWaitInternal(waitNr)
}

func (ring *Ring) Submit() (uint, error) {
	return ring.SubmitAndWaitInternal(0)
}

func (ring *Ring) SubmitAndWaitTimeout(waitNr uint32, timeSpec *syscall.Timespec) (*CompletionQueueEvent, error) {
	var toSubmit uint32

	if timeSpec != nil {
		if ring.features&FeatExtArg > 0 {
			submitted := ring.FlushSQ()
			arg := &getEventsArg{
				sigMask:   uintptr(unsafe.Pointer(nil)),
				sigMaskSz: nSig / szDivider,
				ts:        uintptr(unsafe.Pointer(timeSpec)),
			}
			getData := &getData{
				submit: submitted,
				waitNr: waitNr,
				flags:  EnterExtArg,
				arg:    unsafe.Pointer(arg),
				sz:     int(unsafe.Sizeof(getEventsArg{})),
			}

			cqe, err := ring.getCQEAndEnter(getData)

			runtime.KeepAlive(arg)
			runtime.KeepAlive(getData)
			runtime.KeepAlive(timeSpec)

			return cqe, err
		}

		return nil, ErrNotSupported
	}
	toSubmit = ring.FlushSQ()
	getData := &getData{
		submit: toSubmit,
		waitNr: waitNr,
		flags:  0,
		sz:     nSig / szDivider,
		arg:    unsafe.Pointer(nil),
	}

	return ring.getCQEAndEnter(getData)
}

func (ring *Ring) SQSpaceLeft() uint32 {
	return *ring.sqRing.ringEntries - ring.SQReady()
}

func (ring *Ring) SQReady() uint32 {
	head := *ring.sqRing.head
	if ring.flags&SetupSQPoll > 0 {
		head = atomic.LoadUint32(ring.sqRing.head)
	}

	return ring.sqRing.sqeTail - head
}

//nolint:unused
func (ring *Ring) sqRingWaitInternal() (uint, error) {
	flags := EnterSQWait
	if ring.intFlags&IntFlagRegRing > 0 {
		flags |= EnterRegisteredRing
	}

	return ring.enter(0, 0, flags, nil)
}

//nolint:unused
func (ring *Ring) sqRingWait() (uint, error) {
	if ring.flags&SetupSQPoll == 0 {
		return 0, nil
	}

	if ring.SQSpaceLeft() > 0 {
		return 0, nil
	}

	return ring.sqRingWaitInternal()
}
