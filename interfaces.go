// Copyright (c) 2023 Paweł Gaczyński
// Copyright (c) 2019 Andy Pan
// Copyright (c) 2018 Joshua J Baker
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
	"time"
)

// Reader is an interface that consists of a number of methods for reading that Conn must implement.
type Reader interface {
	io.Reader
	io.WriterTo

	// Next returns a slice containing the next n bytes from the buffer,
	// advancing the buffer as if the bytes had been returned by Read.
	// If n is less or equal to 0, Next returns the entire buffer.
	// The error is io.ErrShortBuffer if n is larger than the reader buffer size.
	//
	// Note that the []byte buf returned by Next() is not allowed to be passed to a new goroutine,
	// as this []byte will be reused within event-loop.
	// If you have to use buf in a new goroutine, then you need to make a copy of buf and pass this copy
	// to that new goroutine.
	Next(n int) (buf []byte, err error)

	// Peek returns the next n bytes without advancing the reader. The bytes stop
	// being valid at the next read call.
	// If n is less or equal to 0, Peek returns the entire buffer.
	//
	// Note that the []byte buf returned by Peek() is not allowed to be passed to a new goroutine,
	// as this []byte will be reused within event-loop.
	// If you have to use buf in a new goroutine, then you need to make a copy of buf and pass this copy
	// to that new goroutine.
	Peek(n int) ([]byte, error)

	// Discard skips the next n bytes, returning the number of bytes discarded.
	//
	// If n > 0 Discards tries to skip n bytes.
	// If n == 0 Discards doesn't skip any bytes.
	// If n < 0 Discards skips all buffered bytes.
	Discard(n int) (int, error)

	// InboundBuffered returns the number of bytes that can be read from the current buffer.
	InboundBuffered() (n int)
}

// Writer is an interface that consists of a number of methods for writing that Conn must implement.
type Writer interface {
	io.Writer
	io.ReaderFrom

	// OutboundBuffered returns the number of bytes that can be read from the current buffer.
	OutboundBuffered() (n int)
}

// Socket is an interface that represents a socket.
type Socket interface {
	// Fd returns underlying file descriptor.
	Fd() int

	// SetReadBuffer sets the size of the operating system's
	// receive buffer associated with the connection.
	SetReadBuffer(bytes int) error

	// SetWriteBuffer sets the size of the operating system's
	// transmit buffer associated with the connection.
	SetWriteBuffer(bytes int) error
	// SetLinger sets the behavior of Close on a connection which still
	// has data waiting to be sent or to be acknowledged.
	//
	// If sec < 0 (the default), the operating system finishes sending the
	// data in the background.
	//
	// If sec == 0, the operating system discards any unsent or
	// unacknowledged data.
	//
	// If sec > 0, the data is sent in the background as with sec < 0. On
	// some operating systems after sec seconds have elapsed any remaining
	// unsent data may be discarded.
	SetLinger(sec int) error

	// SetKeepAlivePeriod tells operating system to send keep-alive messages on the connection
	// and sets period between TCP keep-alive probes.
	SetKeepAlivePeriod(d time.Duration) error

	// SetNoDelay controls whether the operating system should delay
	// packet transmission in hopes of sending fewer packets (Nagle's
	// algorithm).
	// The default is true (no delay), meaning that data is sent as soon as possible after a Write.
	SetNoDelay(noDelay bool) error
}

// Conn is an interface representing a network connection.
// Structures implementing it are not guaranteed to be thread-safe.
// All write operations are asynchronous.
type Conn interface {
	Reader
	Writer
	Socket

	// Context returns a user-defined context.
	Context() (ctx interface{})

	// SetContext sets a user-defined context.
	SetContext(ctx interface{})

	// LocalAddr is the connection's local socket address.
	LocalAddr() (addr net.Addr)

	// RemoteAddr is the connection's remote peer address.
	RemoteAddr() (addr net.Addr)

	// Close closes the current connection.
	Close() error
}
