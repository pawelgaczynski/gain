package gain

import (
	"fmt"
)

type connectionPool struct {
	queue        lockFreeQueue[*connection]
	connections  map[int]*connection
	prefill      bool
	closing      bool
	maxConn      int
	bufferSize   uint
	releaseFdSet map[int]void
}

func (c *connectionPool) get(fileDescriptor int) (*connection, error) {
	conn, ok := c.connections[fileDescriptor]
	if ok {
		return conn, nil
	}

	if c.prefill {
		if c.queue.isEmpty() {
			return nil, fmt.Errorf("connections queue is empty")
		}
		conn = c.queue.dequeue()
		conn.fd = fileDescriptor
		c.connections[fileDescriptor] = conn
	} else {
		if len(c.connections) >= c.maxConn {
			return nil, fmt.Errorf("max connections exceeded")
		}
		if c.queue.isEmpty() {
			buffer := createBuffer(c.bufferSize)
			conn = &connection{
				buffer: buffer,
			}
			conn.fd = fileDescriptor
			c.connections[fileDescriptor] = conn
		} else {
			conn = c.queue.dequeue()
			conn.fd = fileDescriptor
			c.connections[fileDescriptor] = conn
		}
	}

	return conn, nil
}

func (c *connectionPool) release(fileDescriptor int) error {
	conn, ok := c.connections[fileDescriptor]
	if !ok {
		return fmt.Errorf("connection not allocated for fd: %d", fileDescriptor)
	}
	delete(c.connections, fileDescriptor)
	for j := 0; j < len(conn.buffer.data); j++ {
		conn.buffer.data[j] = 0
	}
	c.queue.enqueue(conn)
	delete(c.releaseFdSet, fileDescriptor)
	return nil
}

// cannot be blocked
func (c *connectionPool) close(callback func(conn *connection) error, fdSkipped int) error {
	c.closing = true
	c.releaseFdSet = make(map[int]void)
	for _, value := range c.connections {
		if value.fd == fdSkipped {
			continue
		}
		err := callback(value)
		if err == nil {
			c.releaseFdSet[value.fd] = member
		}
	}
	return nil
}

func (c *connectionPool) allClosed() bool {
	return c.closing && len(c.releaseFdSet) == 0
}

func (c *connectionPool) activeConnections(filter func(c *connection) bool) int {
	count := 0
	for _, conn := range c.connections {
		if filter(conn) {
			count++
		}
	}
	return count
}

func newConnectionPool(maxConn uint, bufferSize uint, prefill bool) *connectionPool {
	allConns := maxConn + 1
	manager := &connectionPool{
		queue:       newConnectionQueue(),
		connections: make(map[int]*connection),
		prefill:     prefill,
		maxConn:     int(allConns),
		bufferSize:  bufferSize,
	}
	if prefill {
		buffers := createBuffers(bufferSize, allConns)
		for _, buffer := range buffers {
			conn := newConnection(buffer)
			manager.queue.enqueue(conn)
		}
	}
	return manager
}
