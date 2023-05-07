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

const (
	SQNeedWakeup uint32 = 1 << iota
	SQCQOverflow
	SQTaskrun
)

const (
	IntFlagRegRing uint8 = 1
)

type SubmissionQueue struct {
	buffer    []byte
	sqeBuffer []byte
	ringSize  uint64

	head        *uint32
	tail        *uint32
	ringMask    *uint32
	ringEntries *uint32
	flags       *uint32
	dropped     *uint32
	array       *uint32

	sqeTail uint32
	sqeHead uint32
}

type CompletionQueue struct {
	buffer   []byte
	ringSize uint64

	head        *uint32
	tail        *uint32
	ringMask    *uint32
	ringEntries *uint32
	overflow    *uint32
	flags       uintptr

	cqeBuff *CompletionQueueEvent
}

type Ring struct {
	sqRing      *SubmissionQueue
	cqRing      *CompletionQueue
	flags       uint32
	fd          int
	features    uint32
	enterRingFd int
	intFlags    uint8
	params      *Params

	exited bool
}

func (ring *Ring) Fd() int {
	return ring.fd
}

func newRing() *Ring {
	return &Ring{
		params: &Params{},
		sqRing: &SubmissionQueue{},
		cqRing: &CompletionQueue{},
	}
}

func CreateRing() (*Ring, error) {
	var (
		ring  = newRing()
		flags uint32
	)

	err := ring.QueueInit(defaultMaxQueue, flags)
	if err != nil {
		return nil, err
	}

	return ring, nil
}
