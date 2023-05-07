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

type connectionManager struct {
	connections  map[int]*connection
	closing      bool
	releaseFdSet map[int]void
	keyPool      *keyPool
}

func (c *connectionManager) fork(conn *connection, write bool) *connection {
	key := int(c.keyPool.get())

	forked := getConnection()
	forked = conn.fork(forked, key, write)
	c.connections[key] = forked

	return forked
}

func (c *connectionManager) getFd(fd int) *connection {
	return c.get(fd, fd)
}

func (c *connectionManager) get(key int, fd int) *connection {
	conn, ok := c.connections[key]
	if ok {
		return conn
	}

	conn = getConnection()
	conn.fd = fd
	conn.key = key
	c.connections[key] = conn

	return conn
}

func (c *connectionManager) release(key int) {
	conn, ok := c.connections[key]
	if ok {
		delete(c.connections, key)
		putConnection(conn)
		delete(c.releaseFdSet, key)
		c.keyPool.put(uint64(conn.key))
	}
}

func (c *connectionManager) close(callback func(conn *connection) bool, fdSkipped int) {
	c.closing = true
	c.releaseFdSet = make(map[int]void)

	for _, value := range c.connections {
		if value.fd == fdSkipped {
			continue
		}

		if callback(value) {
			c.releaseFdSet[value.fd] = member
		}
	}
}

func (c *connectionManager) allClosed() bool {
	return c.closing && len(c.releaseFdSet) == 0
}

func (c *connectionManager) activeConnections(filter func(c *connection) bool) int {
	count := 0

	for _, conn := range c.connections {
		if filter(conn) {
			count++
		}
	}

	return count
}

func newConnectionManager() *connectionManager {
	return &connectionManager{
		connections: make(map[int]*connection),
		keyPool:     newKeyPool(),
	}
}
