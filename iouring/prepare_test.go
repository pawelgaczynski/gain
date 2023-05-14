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
	"testing"

	"github.com/pawelgaczynski/gain/iouring"
	. "github.com/stretchr/testify/require"
)

func TestPrepareMsgRing(t *testing.T) {
	entry := &iouring.SubmissionQueueEntry{}
	entry.PrepareMsgRing(10, 100, 200, 60)

	Equal(t, uint8(40), entry.OpCode)
	Equal(t, int32(10), entry.Fd)
	Equal(t, uint32(100), entry.Len)
	Equal(t, uint64(200), entry.Off)
	Equal(t, uint32(60), entry.OpcodeFlags)
}

func TestPrepareAccept(t *testing.T) {
	entry := &iouring.SubmissionQueueEntry{}
	entry.PrepareAccept(10, 100, 200, 60)

	Equal(t, uint8(13), entry.OpCode)
	Equal(t, int32(10), entry.Fd)
	Equal(t, uint64(100), entry.Addr)
	Equal(t, uint64(200), entry.Off)
	Equal(t, uint32(60), entry.OpcodeFlags)
}

func TestPrepareClose(t *testing.T) {
	entry := &iouring.SubmissionQueueEntry{}
	entry.PrepareClose(10)

	Equal(t, uint8(19), entry.OpCode)
	Equal(t, int32(10), entry.Fd)
}
