package gain

import (
	"fmt"
	"io"
)

type connectionState int

const (
	connInvalid connectionState = iota
	connAccept
	connRead
	connWrite
	connClose
)

func (s connectionState) String() string {
	switch s {
	case connAccept:
		return "accept"
	case connRead:
		return "read"
	case connWrite:
		return "write"
	case connClose:
		return "close"
	default:
		return "invalid"
	}
}

type Conn interface {
	io.Reader
	io.Writer
	Fd() int
}

type connMode int

func (m connMode) String() string {
	switch m {
	case readKernel:
		return "readKernel"
	case readOrWriteUser:
		return "readOrWriteUser"
	case writeUser:
		return "writeUser"
	case writeKernel:
		return "writeKernel"
	default:
		return "invalid"
	}
}

const (
	readKernel = iota
	readOrWriteUser
	writeUser
	writeKernel
)

type connection struct {
	fd     int
	buffer *buffer
	state  connectionState

	index     int
	size      int
	mode      connMode
	anyWrites bool
}

func (c *connection) setReadKernel() {
	c.mode = readKernel
}

func (c *connection) setReadOrWriteUser() {
	c.mode = readOrWriteUser
	c.index = 0
}

func (c *connection) setWriteUser() {
	c.mode = writeUser
}

func (c *connection) setWriteKernel() {
	c.mode = writeKernel
}

func (c *connection) setSize(size int) {
	c.size = size
}

func (c *connection) Fd() int {
	return c.fd
}

func (c *connection) Read(buffer []byte) (int, error) {
	if c.mode != readOrWriteUser {
		return -1, fmt.Errorf("read is not available in mode: %d", c.mode)
	}
	if c.index >= len(c.buffer.data) {
		return -1, io.EOF
	}
	n := copy(buffer, c.buffer.data[c.index:c.size])
	c.index += n
	return n, nil
}

func (c *connection) Write(buffer []byte) (int, error) {
	if c.mode == readOrWriteUser {
		c.index = 0
		c.anyWrites = true
		c.setWriteUser()
	} else if c.mode != writeUser {
		return -1, fmt.Errorf("write is not available in mode: %d", c.mode)
	}
	if c.index+len(buffer) > len(c.buffer.data) {
		return -1, io.EOF
	}
	n := copy(c.buffer.data[c.index:], buffer)
	c.index += n
	return n, nil
}

func newConnection(buffer *buffer) *connection {
	return &connection{
		buffer: buffer,
	}
}
