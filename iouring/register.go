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
	"errors"
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

const (
	RegisterBuffers uint = iota
	UnregisterBuffers

	RegisterFiles
	UnregisterFiles

	RegisterEventFD
	UnregisterEventFD

	RegisterFilesUpdate
	RegisterEventFDAsync
	RegisterProbe

	RegisterPersonality
	UnregisterPersonality

	RegisterRestrictions
	UnregisterEnableRings

	RegisterFiles2
	RegisterFilesUpdate2
	RegisterBuffers2
	RegisterBuffersUpdate

	RegisterIOWQAff
	UnregisterIOWQAff

	RegisterIOWQMaxWorkers

	RegisterRingFDs
	UnregisterRingFDs

	RegisterPbufRing
	UnregisterPbufRing

	RegisterSyncCancel

	RegisterFileAllocRange

	RegisterLast
)

const (
	IOWQBound uint = iota
	IOWQUnbound
)

type (
	FilesUpdate struct {
		Offset uint32
		Resv   uint32
		Fds    uint64
	}

	RsrcRegister struct {
		Nr    uint32
		Resv  uint32
		Resv2 uint32
		Data  uint64
		Tags  uint64
	}

	RsrcUpdate struct {
		Offset uint32
		Resv   uint32
		Data   uint64
	}

	RsrcUpdate2 struct {
		Offset uint32
		Resv   uint32
		Data   uint64
		Tags   uint64
		Nr     uint32
		Resv2  uint32
	}
)

const (
	RsrcRegisterSparse uint32 = 1 << iota
)

const RegisterFilesSkip int = -2

const opSupported uint16 = 1 << 0

func (ring *Ring) RegisterProbe() (*Probe, error) {
	probe := &Probe{}
	_, _, err := ring.Register(RegisterProbe, unsafe.Pointer(probe), probeOpsSize)
	return probe, err
}

func (ring *Ring) RegisterBuffers(iovecs []syscall.Iovec) (uintptr, uintptr, error) {
	return ring.Register(RegisterBuffers, unsafe.Pointer(&iovecs[0]), len(iovecs))
}

func (ring *Ring) UnregisterBuffers() (uintptr, uintptr, error) {
	return ring.Register(UnregisterBuffers, unsafe.Pointer(nil), 0)
}

func (ring *Ring) RegisterIOWQMaxWorkers(args []uint) (uintptr, uintptr, error) {
	return ring.Register(RegisterIOWQMaxWorkers, unsafe.Pointer(&args[0]), regIOWQMaxWorkersNrArgs)
}

func increaseRlimitNofile(nr uint64) error {
	rlim := &syscall.Rlimit{}
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, rlim)
	if err != nil {
		return err
	}

	if rlim.Cur < nr {
		rlim.Cur += nr
		_ = syscall.Setrlimit(syscall.RLIMIT_NOFILE, rlim)
	}

	return nil
}

func (ring *Ring) RegisterFiles(files unsafe.Pointer, nrFiles int) (uintptr, error) {
	var ret uintptr
	var err error
	var didIncrease bool
	for {
		ret, _, err = ring.Register(RegisterFiles, files, nrFiles)
		if err == nil {
			break
		}
		if errors.Is(err, syscall.EMFILE) && !didIncrease {
			didIncrease = true
			_ = increaseRlimitNofile(uint64(nrFiles))
			continue
		}
		break
	}

	return ret, err
}

func (ring *Ring) RegisterFilesSparse(nr uint32) (uintptr, error) {
	ring.flags = RsrcRegisterSparse
	reg := &RsrcRegister{
		Nr: nr,
	}

	var ret uintptr
	var err error
	var didIncrease bool

	for {
		regSize := int(unsafe.Sizeof(RsrcRegister{}))
		ret, _, err = ring.Register(RegisterFiles2, unsafe.Pointer(reg), regSize)
		if err == nil {
			break
		}
		if errors.Is(err, syscall.EMFILE) && !didIncrease {
			didIncrease = true
			_ = increaseRlimitNofile(uint64(nr))
			continue
		}
		break
	}

	runtime.KeepAlive(reg)

	return ret, err
}

func (ring *Ring) RegisterRingFd() (uintptr, error) {
	rsrcUpdate := &RsrcUpdate{
		Data:   uint64(ring.fd),
		Offset: registerRingFdOffset,
	}
	ret, _, err := ring.Register(RegisterRingFDs, unsafe.Pointer(rsrcUpdate), 1)
	if err != nil {
		return ret, err
	}
	if ret == 1 {
		ring.enterRingFd = int(rsrcUpdate.Offset)
		ring.intFlags |= IntFlagRegRing
	}
	return ret, nil
}

func (ring *Ring) UnregisterRingFd() (uintptr, error) {
	rsrcUpdate := &RsrcUpdate{
		Offset: uint32(ring.fd),
	}
	ret, _, err := ring.Register(UnregisterRingFDs, unsafe.Pointer(rsrcUpdate), 1)
	if err != nil {
		return ret, err
	}
	if ret == 1 {
		ring.enterRingFd = int(rsrcUpdate.Offset)
		ring.intFlags &= ^IntFlagRegRing
	}
	return ret, nil
}

func (ring *Ring) Register(op uint, arg unsafe.Pointer, nrArgs int) (uintptr, uintptr, error) {
	returnFirst, returnSecond, errno := syscall.Syscall6(
		sysRegister,
		uintptr(ring.fd),
		uintptr(op),
		uintptr(arg),
		uintptr(nrArgs),
		0,
		0,
	)
	if errno != 0 {
		return 0, 0, os.NewSyscallError("io_uring_register", errno)
	}
	return returnFirst, returnSecond, nil
}
