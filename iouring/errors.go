package iouring

import "errors"

var (
	errNotImplemented     = errors.New("not implemented")
	ErrTimerExpired       = errors.New("timer expired")
	ErrInterrupredSyscall = errors.New("interrupred system call")
	ErrAgain              = errors.New("try again")
)
