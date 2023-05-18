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
	"testing"

	. "github.com/stretchr/testify/require"
)

func TestConvertErrno(t *testing.T) {
	EqualValues(t, ErrTimerExpired, convertErrno(syscall.ETIME))
	EqualValues(t, ErrInterrupredSyscall, convertErrno(syscall.EINTR))
	EqualValues(t, ErrAgain, convertErrno(syscall.EAGAIN))
	EqualValues(t, os.NewSyscallError("io_uring_enter", syscall.EINVAL), convertErrno(syscall.EINVAL))
	EqualValues(t, os.NewSyscallError("io_uring_enter", syscall.EBADF), convertErrno(syscall.EBADF))
	EqualValues(t, os.NewSyscallError("io_uring_enter", syscall.EBUSY), convertErrno(syscall.EBUSY))
}
