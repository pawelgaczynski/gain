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
	EnterGetEvents uint32 = 1 << iota
	EnterSQWakeup
	EnterSQWait
	EnterExtArg
	EnterRegisteredRing
)

func convertErrno(errno syscall.Errno) error {
	switch errno {
	case syscall.ETIME:
		return ErrTimerExpired
	case syscall.EINTR:
		return ErrInterrupredSyscall
	case syscall.EAGAIN:
		return ErrAgain
	default:
		return os.NewSyscallError("io_uring_enter", errno)
	}
}

func (ring *Ring) enter(submitted uint32, waitNr uint32, flags uint32, sig unsafe.Pointer) (uint, error) {
	return ring.enter2(submitted, waitNr, flags, sig, nSig/szDivider)
}

func (ring *Ring) enter2(
	submitted uint32,
	waitNr uint32,
	flags uint32,
	sig unsafe.Pointer,
	size int,
) (uint, error) {
	var (
		consumed uintptr
		errno    syscall.Errno
	)

	consumed, _, errno = syscall.Syscall6(
		sysEnter,
		uintptr(ring.enterRingFd),
		uintptr(submitted),
		uintptr(waitNr),
		uintptr(flags),
		uintptr(sig),
		uintptr(size),
	)

	if errno > 0 {
		return 0, convertErrno(errno)
	}

	return uint(consumed), nil
}
