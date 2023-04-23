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
	"github.com/pawelgaczynski/gain/pkg/pool/ringbuffer"
	"github.com/pawelgaczynski/gain/pkg/pool/sync"
)

var builtinConnectionPool = newConnectionPool()

func getConnection() *connection {
	return builtinConnectionPool.Get()
}

func putConnection(conn *connection) {
	builtinConnectionPool.Put(conn)
}

type connectionPool struct {
	internalPool sync.Pool[*connection]
}

func (c *connectionPool) Put(conn *connection) {
	ringbuffer.Put(conn.inboundBuffer)
	ringbuffer.Put(conn.outboundBuffer)
	conn.inboundBuffer = nil
	conn.outboundBuffer = nil
	conn.fd = 0
	conn.key = 0
	conn.state = 0
	conn.msgHdr = nil
	conn.rawSockaddr = nil
	conn.mode = 0
	conn.ctx = nil

	c.internalPool.Put(conn)
}

func (c *connectionPool) Get() *connection {
	conn := c.internalPool.Get()
	if conn != nil {
		conn.inboundBuffer = ringbuffer.Get()
		conn.outboundBuffer = ringbuffer.Get()

		return conn
	}

	return newConnection()
}

func newConnectionPool() *connectionPool {
	return &connectionPool{
		internalPool: sync.NewPool[*connection](),
	}
}
