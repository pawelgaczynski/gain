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

package gain

import (
	"github.com/pawelgaczynski/gain/iouring"
)

type acceptor struct {
	ring              *iouring.Ring
	socket            int
	clientAddrPointer uintptr
	clientLenPointer  uint64
	connectionPool    *connectionPool
}

func (a *acceptor) addAcceptRequest() (*iouring.SubmissionQueueEntry, error) {
	entry, err := a.ring.GetSQE()
	if err != nil {
		return nil, err
	}
	entry.PrepareAccept(int(uintptr(a.socket)), a.clientAddrPointer, a.clientLenPointer, 0)
	entry.UserData = acceptDataFlag | uint64(a.socket)
	return entry, err
}

func (a *acceptor) addAcceptConnRequest() (*iouring.SubmissionQueueEntry, error) {
	entry, err := a.addAcceptRequest()
	if err != nil {
		return nil, err
	}
	conn, err := a.connectionPool.get(a.socket)
	if err != nil {
		return nil, err
	}
	conn.state = connAccept
	return entry, nil
}

func newAcceptor(ring *iouring.Ring, connectionPool *connectionPool) *acceptor {
	a := &acceptor{
		ring:           ring,
		connectionPool: connectionPool,
	}
	a.clientAddrPointer, a.clientLenPointer = createClientAddr()
	return a
}
