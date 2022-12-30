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

func (ring *Ring) enter(submitted uint32, waitNr uint32, flags uint32, sig unsafe.Pointer, raw bool) (uint, error) {
	return ring.enter2(submitted, waitNr, flags, sig, nSig/szDivider, raw)
}

func (ring *Ring) enter2(
	submitted uint32,
	waitNr uint32,
	flags uint32,
	sig unsafe.Pointer,
	size int,
	raw bool,
) (uint, error) {
	var consumed uintptr
	var errno syscall.Errno
	if raw {
		consumed, _, errno = syscall.RawSyscall6(
			sysEnter,
			uintptr(ring.enterRingFd),
			uintptr(submitted),
			uintptr(waitNr),
			uintptr(flags),
			uintptr(sig),
			uintptr(size),
		)
	} else {
		consumed, _, errno = syscall.Syscall6(
			sysEnter,
			uintptr(ring.enterRingFd),
			uintptr(submitted),
			uintptr(waitNr),
			uintptr(flags),
			uintptr(sig),
			uintptr(size),
		)
	}
	switch errno {
	case syscall.ETIME:
		return 0, ErrTimerExpired
	case syscall.EINTR:
		return 0, ErrInterrupredSyscall
	case syscall.EAGAIN:
		return 0, ErrAgain
	default:
		if errno != 0 {
			return 0, os.NewSyscallError("io_uring_enter", errno)
		}
	}
	return uint(consumed), nil
}
