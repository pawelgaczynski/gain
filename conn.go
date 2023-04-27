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
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/pawelgaczynski/gain/pkg/buffer/magicring"
	"github.com/pawelgaczynski/gain/pkg/errors"
	"github.com/pawelgaczynski/gain/pkg/pool/byteslice"
	"github.com/pawelgaczynski/gain/pkg/pool/ringbuffer"
	"github.com/pawelgaczynski/gain/pkg/socket"
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

func connModeString(m uint32) string {
	switch m {
	case kernelSpace:
		return "readKernel"
	case userSpace:
		return "readOrWriteUser"
	case closed:
		return "closed"
	default:
		return "invalid"
	}
}

const (
	kernelSpace = iota
	userSpace
	closed
)

const (
	msgControlBufferSize = 64
)

const (
	noOp = iota
	readOp
	writeOp
)

type connection struct {
	fd  int
	key int

	inboundBuffer  *magicring.RingBuffer
	outboundBuffer *magicring.RingBuffer
	state          connectionState
	mode           uint32

	msgHdr      *syscall.Msghdr
	rawSockaddr *syscall.RawSockaddrAny

	localAddr  net.Addr
	remoteAddr net.Addr

	ctx interface{}

	nextAsyncOp int
}

func (c *connection) outboundReadAddress() unsafe.Pointer {
	return c.outboundBuffer.ReadAddress()
}

func (c *connection) inboundWriteAddress() unsafe.Pointer {
	return c.inboundBuffer.WriteAddress()
}

func (c *connection) setKernelSpace() {
	atomic.StoreUint32(&c.mode, kernelSpace)
}

func (c *connection) setUserSpace() {
	atomic.StoreUint32(&c.mode, userSpace)
}

func (c *connection) Context() interface{} {
	return c.ctx
}

func (c *connection) SetContext(ctx interface{}) {
	c.ctx = ctx
}

func (c *connection) LocalAddr() net.Addr {
	return c.localAddr
}

func (c *connection) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func (c *connection) Fd() int {
	return c.fd
}

func (c *connection) onKernelRead(n int) {
	c.inboundBuffer.AdvanceWrite(n)
}

func (c *connection) onKernelWrite(n int) {
	c.outboundBuffer.AdvanceRead(n)
}

func (c *connection) isClosed() bool {
	return atomic.LoadUint32(&c.mode) == closed
}

func (c *connection) Close() error {
	if mode := atomic.LoadUint32(&c.mode); mode == closed {
		return errors.ErrConnectionAlreadyClosed
	}

	atomic.StoreUint32(&c.mode, closed)

	return nil
}

func (c *connection) Next(n int) ([]byte, error) {
	if mode := atomic.LoadUint32(&c.mode); mode != userSpace {
		return nil, errors.ErrorOpNotAvailableInMode("next", connModeString(mode))
	}

	bytes, err := c.inboundBuffer.Next(n)
	if err != nil {
		return nil, fmt.Errorf("connection inbound buffer Next error: %w", err)
	}

	return bytes, nil
}

func (c *connection) Discard(n int) (int, error) {
	if mode := atomic.LoadUint32(&c.mode); mode != userSpace {
		return 0, errors.ErrorOpNotAvailableInMode("discard", connModeString(mode))
	}

	return c.inboundBuffer.Discard(n), nil
}

func (c *connection) Peek(n int) ([]byte, error) {
	if mode := atomic.LoadUint32(&c.mode); mode != userSpace {
		return nil, errors.ErrorOpNotAvailableInMode("discard", connModeString(mode))
	}

	return c.inboundBuffer.Peek(n), nil
}

func (c *connection) ReadFrom(r io.Reader) (int64, error) {
	if mode := atomic.LoadUint32(&c.mode); mode != userSpace {
		return 0, errors.ErrorOpNotAvailableInMode("readFrom", connModeString(mode))
	}

	bytesRead, err := c.outboundBuffer.ReadFrom(r)
	if err != nil {
		return 0, fmt.Errorf("connection outbound buffer ReadFrom error: %w", err)
	}

	return bytesRead, nil
}

func (c *connection) WriteTo(w io.Writer) (int64, error) {
	if mode := atomic.LoadUint32(&c.mode); mode != userSpace {
		return 0, errors.ErrorOpNotAvailableInMode("writeTo", connModeString(mode))
	}

	bytesWritten, err := c.inboundBuffer.WriteTo(w)
	if err != nil {
		return 0, fmt.Errorf("connection inbound buffer WriteTo error: %w", err)
	}

	return bytesWritten, nil
}

func (c *connection) Read(buffer []byte) (int, error) {
	if mode := atomic.LoadUint32(&c.mode); mode != userSpace {
		return -1, errors.ErrorOpNotAvailableInMode("read", connModeString(mode))
	}

	bytesRead, err := c.inboundBuffer.Read(buffer)
	if err != nil {
		return 0, fmt.Errorf("connection inbound buffer Read error: %w", err)
	}

	return bytesRead, nil
}

func (c *connection) Write(buffer []byte) (int, error) {
	if mode := atomic.LoadUint32(&c.mode); mode != userSpace {
		return 0, errors.ErrorOpNotAvailableInMode("write", connModeString(mode))
	}

	bytesWritten, err := c.outboundBuffer.Write(buffer)
	if err != nil {
		return 0, fmt.Errorf("connection outbound buffer Write error: %w", err)
	}

	return bytesWritten, nil
}

func (c *connection) OutboundBuffered() int {
	return c.outboundBuffer.Buffered()
}

func (c *connection) InboundBuffered() int {
	return c.inboundBuffer.Buffered()
}

func (c *connection) setMsgHeaderWrite() {
	c.msgHdr.Iov.Base = (*byte)(c.outboundReadAddress())
	c.msgHdr.Iov.SetLen(c.OutboundBuffered())
}

func (c *connection) initMsgHeader() {
	var iovec syscall.Iovec
	iovec.Base = (*byte)(c.inboundWriteAddress())
	iovec.SetLen(c.inboundBuffer.Cap())

	var (
		msg syscall.Msghdr
		rsa syscall.RawSockaddrAny
	)

	msg.Name = (*byte)(unsafe.Pointer(&rsa))
	msg.Namelen = uint32(syscall.SizeofSockaddrAny)
	msg.Iov = &iovec
	msg.Iovlen = 1

	controlBuffer := byteslice.Get(msgControlBufferSize)
	msg.Control = (*byte)(unsafe.Pointer(&controlBuffer[0]))
	msg.SetControllen(msgControlBufferSize)

	c.msgHdr = &msg
	c.rawSockaddr = &rsa
}

func (c *connection) fork(newConn *connection, key int, write bool) *connection {
	newConn.inboundBuffer = c.inboundBuffer
	newConn.outboundBuffer = c.outboundBuffer
	newConn.msgHdr = c.msgHdr
	newConn.rawSockaddr = c.rawSockaddr
	newConn.state = c.state
	newConn.fd = c.fd
	newConn.key = key

	if sockAddr, err := anyToSockaddr(newConn.rawSockaddr); err == nil {
		newConn.remoteAddr = socket.SockaddrToUDPAddr(sockAddr)
	}

	if write {
		newConn.setMsgHeaderWrite()
	}

	c.inboundBuffer = ringbuffer.Get()
	c.outboundBuffer = ringbuffer.Get()
	c.initMsgHeader()

	return newConn
}

func newConnection() *connection {
	conn := &connection{
		inboundBuffer:  ringbuffer.Get(),
		outboundBuffer: ringbuffer.Get(),
	}

	return conn
}
