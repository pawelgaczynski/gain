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
	"os"
	"syscall"
	"unsafe"
)

const (
	SetupIOPoll uint32 = 1 << iota
	SetupSQPoll
	SetupSQAff
	SetupSQSize
	SetupClamp
	SetupAttachWQ
	SetupRDisabled
	SetupSubmitAll
	SetupCoopTaskrun
	SetupTaskrunFlag
	SetupSQE128
	SetupSQE32
	SetupSingleIssuer
	SetupDeferTaskrun
)

const (
	FeatSingleMMap uint32 = 1 << iota
	FeatNoDrop
	FeatSubmitStable
	FeatRWCurPos
	FeatCurPersonality
	FeatFastPoll
	FeatPoll32Bits
	FeatSQPollNonfixed
	FeatExtArg
	FeatNativeWorkers
	FeatRcrcTags
	FeatCQESkip
	FeatLinkedFile
)

type SQRingOffsets struct {
	head        uint32
	tail        uint32
	ringMask    uint32
	ringEntries uint32
	flags       uint32
	dropped     uint32
	array       uint32
	resv1       uint32
	resv2       uint64
}

type CQRingOffsets struct {
	head        uint32
	tail        uint32
	ringMask    uint32
	ringEntries uint32
	overflow    uint32
	cqes        uint32
	flags       uint32
	resv1       uint32
	resv2       uint64
}

type Params struct {
	sqEntries    uint32
	cqEntries    uint32
	flags        uint32
	sqThreadCPU  uint32
	sqThreadIdle uint32
	features     uint32
	wqFD         uint32
	resv         [3]uint32

	sqOff SQRingOffsets
	cqOff CQRingOffsets
}

func (ring *Ring) QueueInitParams(entries uint) error {
	fd, _, errno := syscall.Syscall(sysSetup, uintptr(entries), uintptr(unsafe.Pointer(ring.params)), 0)
	if errno != 0 {
		return os.NewSyscallError("io_uring_setup", errno)
	}

	fileDescriptor := int(fd)

	err := ring.mmap(fileDescriptor)
	if err != nil {
		return err
	}

	ring.features = ring.params.features
	ring.fd = fileDescriptor
	ring.enterRingFd = fileDescriptor
	ring.flags = ring.params.flags

	return nil
}

func (ring *Ring) QueueInit(entries uint, flags uint32) error {
	ring.params.flags = flags

	return ring.QueueInitParams(entries)
}

func (ring *Ring) Close() error {
	if ring.fd != 0 {
		return os.NewSyscallError("close", syscall.Close(ring.fd))
	}

	return nil
}

func (ring *Ring) QueueExit() error {
	ring.exited = true

	err := ring.munmap()
	if err != nil {
		return err
	}

	ring.UnmapRings()

	err = ring.Close()
	if err != nil {
		return err
	}

	return nil
}
