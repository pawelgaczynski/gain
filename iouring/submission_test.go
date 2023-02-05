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

package iouring_test

import (
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/pawelgaczynski/gain/iouring"
	. "github.com/stretchr/testify/require"
)

func TestSubmitAndWait(t *testing.T) {
	ring, err := iouring.CreateRing()
	NoError(t, err)
	defer ring.Close()

	cqeBuff := make([]*iouring.CompletionQueueEvent, 16)

	cnt := ring.PeekBatchCQE(cqeBuff)
	Equal(t, 0, cnt)

	NoError(t, queueNOPs(t, ring, 4, 0))

	timespec := syscall.NsecToTimespec((time.Millisecond * 100).Nanoseconds())
	_, err = ring.SubmitAndWaitTimeout(10, &timespec)
	runtime.KeepAlive(timespec)

	NoError(t, err)
}

func TestSubmitAndWaitNilTimeout(t *testing.T) {
	ring, err := iouring.CreateRing()
	NoError(t, err)
	defer ring.Close()

	cqeBuff := make([]*iouring.CompletionQueueEvent, 16)

	cnt := ring.PeekBatchCQE(cqeBuff)
	Equal(t, 0, cnt)

	NoError(t, queueNOPs(t, ring, 4, 0))

	_, err = ring.SubmitAndWaitTimeout(1, nil)
	NoError(t, err)
}
