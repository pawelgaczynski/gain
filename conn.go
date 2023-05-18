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
	"io"
	"net"
	"sync/atomic"
	"syscall"
	"time"
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

const (
	kernelSpace = iota
	userSpace
)

func connModeString(m uint32) string {
	switch m {
	case kernelSpace:
		return "kernelSpace"
	case userSpace:
		return "userSpace"
	default:
		return "invalid"
	}
}

const (
	msgControlBufferSize = 64
)

const (
	noOp = iota
	readOp
	writeOp
	closeOp
)

const (
	tcp = iota
	udp
)

type connection struct {
	fd      int
	key     int
	network uint32

	inboundBuffer  *magicring.RingBuffer
	outboundBuffer *magicring.RingBuffer
	state          connectionState
	mode           atomic.Uint32
	closed         atomic.Bool

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
	c.mode.Store(kernelSpace)
}

func (c *connection) setUserSpace() {
	c.mode.Store(userSpace)
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

func (c *connection) userOpAllowed(name string) error {
	if c.closed.Load() {
		return errors.ErrConnectionClosed
	}

	if mode := c.mode.Load(); mode != userSpace {
		return errors.ErrorOpNotAvailableInMode(name, connModeString(mode))
	}

	return nil
}

func (c *connection) SetReadBuffer(bytes int) error {
	err := c.userOpAllowed("setReadBuffer")
	if err != nil {
		return err
	}
	//nolint:wrapcheck
	return socket.SetRecvBuffer(c.fd, bytes)
}

func (c *connection) SetWriteBuffer(bytes int) error {
	err := c.userOpAllowed("setWriteBuffer")
	if err != nil {
		return err
	}
	//nolint:wrapcheck
	return socket.SetSendBuffer(c.fd, bytes)
}

func (c *connection) SetLinger(sec int) error {
	err := c.userOpAllowed("setLinger")
	if err != nil {
		return err
	}
	//nolint:wrapcheck
	return socket.SetLinger(c.fd, sec)
}

func (c *connection) SetNoDelay(noDelay bool) error {
	err := c.userOpAllowed("setNoDelay")
	if err != nil {
		return err
	}
	//nolint:wrapcheck
	return socket.SetNoDelay(c.fd, boolToInt(noDelay))
}

func (c *connection) SetKeepAlivePeriod(period time.Duration) error {
	err := c.userOpAllowed("setKeepAlivePeriod")
	if err != nil {
		return err
	}
	//nolint:wrapcheck
	return socket.SetKeepAlivePeriod(c.fd, int(period.Seconds()))
}

func (c *connection) onKernelRead(n int) {
	c.inboundBuffer.AdvanceWrite(n)
}

func (c *connection) onKernelWrite(n int) {
	c.outboundBuffer.AdvanceRead(n)
}

func (c *connection) isClosed() bool {
	return c.closed.Load()
}

func (c *connection) Close() error {
	if network := atomic.LoadUint32(&c.network); network == udp {
		return nil
	}

	if c.closed.Load() {
		return errors.ErrConnectionAlreadyClosed
	}

	c.closed.Store(true)

	return nil
}

func (c *connection) Next(n int) ([]byte, error) {
	err := c.userOpAllowed("next")
	if err != nil {
		return nil, err
	}

	//nolint:wrapcheck
	return c.inboundBuffer.Next(n)
}

func (c *connection) Discard(n int) (int, error) {
	err := c.userOpAllowed("discard")
	if err != nil {
		return 0, err
	}

	return c.inboundBuffer.Discard(n), nil
}

func (c *connection) Peek(n int) ([]byte, error) {
	err := c.userOpAllowed("peek")
	if err != nil {
		return nil, err
	}

	return c.inboundBuffer.Peek(n), nil
}

func (c *connection) ReadFrom(reader io.Reader) (int64, error) {
	err := c.userOpAllowed("readFrom")
	if err != nil {
		return 0, err
	}

	//nolint:wrapcheck
	return c.outboundBuffer.ReadFrom(reader)
}

func (c *connection) WriteTo(writer io.Writer) (int64, error) {
	err := c.userOpAllowed("writeTo")
	if err != nil {
		return 0, err
	}

	//nolint:wrapcheck
	return c.inboundBuffer.WriteTo(writer)
}

func (c *connection) Read(buffer []byte) (int, error) {
	err := c.userOpAllowed("read")
	if err != nil {
		return 0, err
	}

	//nolint:wrapcheck
	return c.inboundBuffer.Read(buffer)
}

func (c *connection) Write(buffer []byte) (int, error) {
	err := c.userOpAllowed("write")
	if err != nil {
		return 0, err
	}

	//nolint:wrapcheck
	return c.outboundBuffer.Write(buffer)
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
	newConn.network = udp

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
