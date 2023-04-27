// Copyright (c) 2023 Paweł Gaczyński
// Copyright (c) 2019 Chao yuepan, Andy Pan, Allen Xu
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE

package magicring

import (
	"fmt"
	"io"
	"log"
	"os"
	"unsafe"

	"github.com/pawelgaczynski/gain/pkg/errors"
	"github.com/pawelgaczynski/gain/pkg/pool/virtualmem"
)

var (
	DefaultMagicBufferSize = os.Getpagesize()
	MinRead                = 1024
)

// RingBuffer is a circular buffer that implement io.ReaderWriter interface.
type RingBuffer struct {
	vm *virtualmem.VirtualMem

	Size    int
	r       int // next position to read
	w       int // next position to write
	isEmpty bool
}

func (rb *RingBuffer) ReadAddress() unsafe.Pointer {
	return unsafe.Pointer(&rb.vm.Buf[rb.r])
}

func (rb *RingBuffer) WriteAddress() unsafe.Pointer {
	return unsafe.Pointer(&rb.vm.Buf[rb.w])
}

func (rb *RingBuffer) Zeroes() {
	for j := 0; j < rb.Size; j++ {
		rb.vm.Buf[j] = 0
	}
}

func (rb *RingBuffer) ReleaseBytes() {
	virtualmem.Put(rb.vm)
}

func (rb *RingBuffer) Reset() {
	rb.isEmpty = true
	rb.r, rb.w = 0, 0
}

func (rb *RingBuffer) Read(buffer []byte) (int, error) {
	if len(buffer) == 0 {
		return 0, nil
	}

	if rb.isEmpty {
		return 0, errors.ErrIsEmpty
	}

	bytesToWrite := rb.Buffered()
	if bytesToWrite > len(buffer) {
		bytesToWrite = len(buffer)
	}

	copy(buffer, rb.vm.Buf[rb.r:rb.r+bytesToWrite])

	rb.r = (rb.r + bytesToWrite) % rb.Size
	if rb.r == rb.w {
		rb.Reset()
	}

	return bytesToWrite, nil
}

func (rb *RingBuffer) Write(buffer []byte) (int, error) {
	bytesToRead := len(buffer)
	if bytesToRead == 0 {
		return bytesToRead, nil
	}

	free := rb.Available()
	if bytesToRead > free {
		rb.Grow(rb.Size + bytesToRead - free)
	}

	copy(rb.vm.Buf[rb.w:], buffer)
	rb.w = (rb.w + bytesToRead) % rb.Size
	rb.isEmpty = false

	return bytesToRead, nil
}

func (rb *RingBuffer) AdvanceWrite(bytesToAdvance int) {
	if bytesToAdvance == 0 {
		return
	}
	free := rb.Available()
	rb.isEmpty = false

	if bytesToAdvance > free {
		log.Panicf("AdvanceWrite - bytesToAdvance (%d) is too large (free/size: %d/%d)", bytesToAdvance, free, rb.Size)
	}

	if rb.w+bytesToAdvance >= rb.Size {
		m := rb.w + bytesToAdvance - rb.Size
		rb.w = m
	} else {
		rb.w += bytesToAdvance
	}
}

func (rb *RingBuffer) AdvanceRead(bytesToAdvance int) {
	if bytesToAdvance == 0 {
		return
	}

	buffered := rb.Buffered()
	if bytesToAdvance > buffered {
		log.Panic("n is too large: ", bytesToAdvance)
	}

	if rb.r+bytesToAdvance >= rb.Size {
		m := rb.r + bytesToAdvance - rb.Size
		rb.r = m
	} else {
		rb.r += bytesToAdvance
	}

	if rb.r == rb.w {
		rb.Reset()
	}
}

func (rb *RingBuffer) Grow(newCap int) {
	newCap = virtualmem.AdjustBufferSize(newCap)
	newBuf := virtualmem.Get(newCap)
	oldLen := rb.Buffered()
	_, _ = rb.Read(newBuf.Buf)

	if rb.vm != nil && rb.vm.Buf != nil {
		rb.ReleaseBytes()
	}
	rb.vm = newBuf
	rb.r = 0
	rb.w = oldLen
	rb.Size = newCap

	if rb.w > 0 {
		rb.isEmpty = false
	}
}

// Available returns the length of available bytes to write.
func (rb *RingBuffer) Available() int {
	if rb.r == rb.w {
		if rb.isEmpty {
			return rb.Size
		}

		return 0
	}

	if rb.w < rb.r {
		return rb.r - rb.w
	}

	return rb.Size - rb.w + rb.r
}

// Buffered returns the length of available bytes to read.
func (rb *RingBuffer) Buffered() int {
	if rb.r == rb.w {
		if rb.isEmpty {
			return 0
		}

		return rb.Size
	}

	if rb.w > rb.r {
		return rb.w - rb.r
	}

	return rb.Size - rb.r + rb.w
}

// IsFull tells if this ring-buffer is full.
func (rb *RingBuffer) IsFull() bool {
	return rb.r == rb.w && !rb.isEmpty
}

// IsEmpty tells if this ring-buffer is empty.
func (rb *RingBuffer) IsEmpty() bool {
	return rb.isEmpty
}

// Cap returns the size of the underlying buffer.
func (rb *RingBuffer) Cap() int {
	return rb.Size
}

func (rb *RingBuffer) Next(nBytes int) ([]byte, error) {
	bufferLen := rb.Buffered()
	if nBytes > bufferLen {
		return nil, io.ErrShortBuffer
	} else if nBytes <= 0 {
		nBytes = bufferLen
	}

	defer func() {
		_ = rb.Discard(nBytes)
	}()

	return rb.vm.Buf[rb.r : rb.r+nBytes], nil
}

func (rb *RingBuffer) Peek(bytesToPeak int) []byte {
	var buffer []byte
	if rb.isEmpty {
		return buffer
	}

	if bytesToPeak <= 0 {
		return rb.peekAll()
	}

	bufferedBytes := rb.Buffered() // length of ring-buffer
	if bufferedBytes > bytesToPeak {
		bufferedBytes = bytesToPeak
	}

	return rb.vm.Buf[rb.r : rb.r+bufferedBytes]
}

func (rb *RingBuffer) peekAll() []byte {
	var buffer []byte
	if rb.isEmpty {
		return buffer
	}

	return rb.vm.Buf[rb.r : rb.r+rb.Buffered()]
}

// Discard skips the next n bytes by advancing the read pointer.
func (rb *RingBuffer) Discard(bytesToDiscard int) int {
	var discarded int

	if bytesToDiscard == 0 {
		return 0
	}

	discarded = rb.Buffered()
	if bytesToDiscard < discarded {
		rb.r = (rb.r + bytesToDiscard) % rb.Size

		return bytesToDiscard
	}

	rb.Reset()

	return discarded
}

// Bytes returns all available read bytes. It does not move the read pointer and only copy the available data.
func (rb *RingBuffer) Bytes() []byte {
	if rb.isEmpty {
		return nil
	}
	buffer := make([]byte, rb.Buffered())
	copy(buffer, rb.vm.Buf[rb.r:rb.r+rb.Buffered()])

	return buffer
}

func (rb *RingBuffer) WriteByte(c byte) error {
	if rb.Available() < 1 {
		rb.Grow(rb.Size + 1)
	}
	rb.vm.Buf[rb.w] = c

	rb.w++
	if rb.w == rb.Size {
		rb.w = 0
	}
	rb.isEmpty = false

	return nil
}

// ReadByte reads and returns the next byte from the input or ErrIsEmpty.
func (rb *RingBuffer) ReadByte() (byte, error) {
	if rb.isEmpty {
		return 0, errors.ErrIsEmpty
	}
	byteRead := rb.vm.Buf[rb.r]

	rb.r++
	if rb.r == rb.Size {
		rb.r = 0
	}

	if rb.r == rb.w {
		rb.Reset()
	}

	return byteRead, nil
}

func (rb *RingBuffer) GrowIfUnsufficientFreeSpace() {
	if rb.Available() < MinRead {
		rb.Grow(rb.Buffered() + MinRead)
	}
}

// ReadFrom implements io.ReaderFrom.
func (rb *RingBuffer) ReadFrom(reader io.Reader) (int64, error) {
	var (
		totalBytesRead int64
		bytesRead      int
		err            error
	)

	for {
		rb.GrowIfUnsufficientFreeSpace()

		bytesRead, err = reader.Read(rb.vm.Buf[rb.w : rb.w+rb.Available()])
		if bytesRead < 0 {
			log.Panic("RingBuffer.ReadFrom: reader returned negative count from Read")
		}
		rb.isEmpty = false
		rb.w = (rb.w + bytesRead) % rb.Size

		totalBytesRead += int64(bytesRead)
		if err == io.EOF {
			return totalBytesRead, nil
		}

		if err != nil {
			return totalBytesRead, fmt.Errorf("reader Read error: %w", err)
		}
	}
}

// WriteTo implements io.WriterTo.
func (rb *RingBuffer) WriteTo(writer io.Writer) (int64, error) {
	if rb.isEmpty {
		return 0, errors.ErrIsEmpty
	}
	bufferedBytes := rb.Buffered()

	bytesWritten, err := writer.Write(rb.vm.Buf[rb.r : rb.r+bufferedBytes])
	if bytesWritten > bufferedBytes {
		log.Panicf("RingBuffer.WriteTo: invalid Write count [m > n | m: %d, n: %d]", bytesWritten, bufferedBytes)
	}

	rb.r = (rb.r + bytesWritten) % rb.Size
	if rb.r == rb.w {
		rb.Reset()
	}

	if err != nil {
		return int64(bytesWritten), err
	}

	if !rb.isEmpty {
		return int64(bytesWritten), io.ErrShortWrite
	}

	return int64(bytesWritten), nil
}

// New returns a new Buffer whose buffer has the given size.
func NewMagicBuffer(size int) *RingBuffer {
	if size == 0 {
		return &RingBuffer{isEmpty: true}
	}
	size = virtualmem.AdjustBufferSize(size)
	buffer := &RingBuffer{
		vm:      virtualmem.Get(size),
		Size:    size,
		isEmpty: true,
	}

	return buffer
}
