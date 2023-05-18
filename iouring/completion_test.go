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
	"testing"

	. "github.com/stretchr/testify/require"
)

func queueNOPs(t *testing.T, ring *Ring, number int, offset int) error {
	t.Helper()

	for i := 0; i < number; i++ {
		entry, err := ring.GetSQE()
		if err != nil {
			return fmt.Errorf("error getting SQE: %w", err)
		}

		entry.PrepareNop()
		entry.UserData = uint64(i + offset)
	}
	submitted, err := ring.Submit()
	Equal(t, int(submitted), number)

	return err
}

func TestPeekBatchCQE(t *testing.T) {
	ring, err := CreateRing()
	NoError(t, err)

	defer ring.Close()

	cqeBuff := make([]*CompletionQueueEvent, 16)

	cnt := ring.PeekBatchCQE(cqeBuff)
	Equal(t, 0, cnt)

	NoError(t, queueNOPs(t, ring, 4, 0))

	cnt = ring.PeekBatchCQE(cqeBuff)
	Equal(t, 4, cnt)

	for i := 0; i < 4; i++ {
		Equal(t, uint64(i), cqeBuff[i].UserData())
	}

	NoError(t, queueNOPs(t, ring, 4, 4))

	ring.CQAdvance(uint32(4))
	cnt = ring.PeekBatchCQE(cqeBuff)
	Equal(t, 4, cnt)

	for i := 0; i < 4; i++ {
		Equal(t, uint64(i+4), cqeBuff[i].UserData())
	}

	ring.CQAdvance(uint32(4))
}

func TestCQEFlagsString(t *testing.T) {
	var cqe CompletionQueueEvent

	cqe.flags = CQEFBuffer
	Equal(t, "CQEFBuffer", cqe.FlagsString())

	cqe.flags = CQEFMore
	Equal(t, "CQEFMore", cqe.FlagsString())

	cqe.flags = CQEFSockNonempty
	Equal(t, "CQEFSockNonempty", cqe.FlagsString())

	cqe.flags = CQEFNotif
	Equal(t, "CQEFNotif", cqe.FlagsString())

	cqe.flags = CQEFBuffer | CQEFMore | CQEFSockNonempty | CQEFNotif
	Equal(t, "CQEFBuffer | CQEFMore | CQEFSockNonempty | CQEFNotif", cqe.FlagsString())
}

func TestCQEGetters(t *testing.T) {
	var cqe CompletionQueueEvent
	cqe.flags = 123
	cqe.res = 45
	cqe.userData = 6789

	Equal(t, uint32(123), cqe.Flags())
	Equal(t, int32(45), cqe.Res())
	Equal(t, uint64(6789), cqe.UserData())
}

func TestRingCqRingNeedsEnter(t *testing.T) {
	var ring Ring
	ring.sqRing = &SubmissionQueue{}

	var flags uint32
	ring.sqRing.flags = &flags

	False(t, ring.cqRingNeedsEnter())

	ring.flags |= SetupIOPoll

	True(t, ring.cqRingNeedsEnter())

	ring.flags = 0

	False(t, ring.cqRingNeedsEnter())

	flags |= SQCQOverflow

	True(t, ring.cqRingNeedsEnter())
}
