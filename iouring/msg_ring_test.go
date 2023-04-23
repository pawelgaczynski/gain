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

func TestMsgRingItself(t *testing.T) {
	ring, err := iouring.CreateRing()
	Nil(t, err)

	defer ring.Close()

	entry, err := ring.GetSQE()
	Nil(t, err)
	entry.PrepareMsgRing(ring.Fd(), 100, 200, 0)
	entry.UserData = 123

	entry, err = ring.GetSQE()
	Nil(t, err)
	entry.PrepareMsgRing(ring.Fd(), 300, 400, 0)
	entry.UserData = 234

	entry, err = ring.GetSQE()
	Nil(t, err)
	entry.PrepareMsgRing(ring.Fd(), 500, 600, 0)
	entry.UserData = 345

	numberOfCQEsSubmitted, err := ring.Submit()
	Nil(t, err)
	Equal(t, uint(3), numberOfCQEsSubmitted)

	cqes := make([]*iouring.CompletionQueueEvent, 128)

	numberOfCQEs := ring.PeekBatchCQE(cqes)
	Equal(t, 6, numberOfCQEs)

	cqe := cqes[0]
	Equal(t, uint64(200), cqe.UserData())
	Equal(t, int32(100), cqe.Res())

	cqe = cqes[1]
	Equal(t, uint64(400), cqe.UserData())
	Equal(t, int32(300), cqe.Res())

	cqe = cqes[2]
	Equal(t, uint64(600), cqe.UserData())
	Equal(t, int32(500), cqe.Res())

	cqe = cqes[3]
	Equal(t, uint64(123), cqe.UserData())
	Equal(t, int32(0), cqe.Res())

	cqe = cqes[4]
	Equal(t, uint64(234), cqe.UserData())
	Equal(t, int32(0), cqe.Res())

	cqe = cqes[5]
	Equal(t, uint64(345), cqe.UserData())
	Equal(t, int32(0), cqe.Res())

	ring.CQAdvance(uint32(numberOfCQEs))
}

func TestMsgRing(t *testing.T) {
	senderRing, err := iouring.CreateRing()
	Nil(t, err)

	defer senderRing.Close()

	receiverRing, err := iouring.CreateRing()
	Nil(t, err)

	defer receiverRing.Close()

	entry, err := senderRing.GetSQE()
	Nil(t, err)
	entry.PrepareMsgRing(receiverRing.Fd(), 100, 200, 0)

	entry, err = senderRing.GetSQE()
	Nil(t, err)
	entry.PrepareMsgRing(receiverRing.Fd(), 300, 400, 0)

	entry, err = senderRing.GetSQE()
	Nil(t, err)
	entry.PrepareMsgRing(receiverRing.Fd(), 500, 600, 0)

	cqeNr, err := senderRing.Submit()
	Nil(t, err)
	Equal(t, uint(3), cqeNr)

	cqes := make([]*iouring.CompletionQueueEvent, 128)

	numberOfCQEs := receiverRing.PeekBatchCQE(cqes)
	Equal(t, 3, numberOfCQEs)

	cqe := cqes[0]
	Equal(t, uint64(200), cqe.UserData())
	Equal(t, int32(100), cqe.Res())

	cqe = cqes[1]
	Equal(t, uint64(400), cqe.UserData())
	Equal(t, int32(300), cqe.Res())

	cqe = cqes[2]
	Equal(t, uint64(600), cqe.UserData())
	Equal(t, int32(500), cqe.Res())
	receiverRing.CQAdvance(uint32(numberOfCQEs))
}
