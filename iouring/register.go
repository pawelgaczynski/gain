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
