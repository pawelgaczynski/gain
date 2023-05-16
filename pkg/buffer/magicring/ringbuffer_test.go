// Copyright (c) 2023 Paweł Gaczyński
// Copyright (c) 2019 Chao yuepan, Andy Pan
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
	"bytes"
	"crypto/rand"
	"strings"
	"testing"

	"github.com/pawelgaczynski/gain/pkg/errors"
	. "github.com/stretchr/testify/require"
)

func BenchmarkMagicRingBufferWrite(b *testing.B) {
	ringBuffer := NewMagicBuffer(DefaultMagicBufferSize)
	firstData := []byte(strings.Repeat("abcd", DefaultMagicBufferSize/8))
	secondData := []byte(strings.Repeat("defg", DefaultMagicBufferSize/4))
	buf := make([]byte, DefaultMagicBufferSize)
	_, _ = ringBuffer.Write(firstData)

	for i := 0; i < b.N; i++ {
		_, _ = ringBuffer.Write(secondData)
		_, _ = ringBuffer.Read(buf)
	}
}

func TestMagicRingBufferMultipleWrites(t *testing.T) {
	ringBuffer := NewMagicBuffer(DefaultMagicBufferSize)

	firstData := []byte(strings.Repeat("a", DefaultMagicBufferSize/2))
	secondData := []byte(strings.Repeat("a", DefaultMagicBufferSize))
	bytesWritten, err := ringBuffer.Write(firstData)
	EqualValuesf(t, DefaultMagicBufferSize/2, bytesWritten,
		"expect writes %d bytes but got %d", DefaultMagicBufferSize/2, bytesWritten)
	NoError(t, err, "failed to write data")

	for i := 0; i < 1024; i++ {
		bytesWritten, err = ringBuffer.Write(secondData)
		NoError(t, err, "failed to write data")
		EqualValuesf(t, DefaultMagicBufferSize, bytesWritten,
			"expect writes %d bytes but got %d", DefaultMagicBufferSize, bytesWritten)
	}
}

func TestMagicRingBuffer64MBSize(t *testing.T) {
	rb := NewMagicBuffer(DefaultMagicBufferSize)
	oneMB := 1024 * 1024
	data := []byte(strings.Repeat("a", oneMB))

	for i := 0; i < 64; i++ {
		n, err := rb.Write(data)
		EqualValuesf(t, oneMB, n, "expect writes %d bytes but got %d", oneMB, n)
		NoError(t, err, "failed to write data")
	}
}

func TestMagicRingBufferNext(t *testing.T) {
	ringBuffer := NewMagicBuffer(64)
	True(t, ringBuffer.IsEmpty(), "expect IsEmpty is true but got false")
	False(t, ringBuffer.IsFull(), "expect IsFull is false but got true")
	EqualValuesf(t, 0, ringBuffer.Buffered(), "expect len 0 bytes but got %d. r.w=%d, r.r=%d",
		ringBuffer.Buffered(), ringBuffer.w, ringBuffer.r)
	EqualValuesf(t, DefaultMagicBufferSize, ringBuffer.Available(),
		"expect free %d bytes but got %d. r.w=%d, r.r=%d",
		DefaultMagicBufferSize, ringBuffer.Available(), ringBuffer.w, ringBuffer.r)

	data := make([]byte, 32)
	bytesRead, err := rand.Read(data)
	EqualValues(t, 32, bytesRead)
	NoError(t, err)

	bytesWritten, err := ringBuffer.Write(data)
	False(t, ringBuffer.IsEmpty(), "expect IsEmpty is false but got true")
	False(t, ringBuffer.IsFull(), "expect IsFull is false but got true")
	EqualValues(t, 32, bytesWritten)
	NoError(t, err)

	buffer, err := ringBuffer.Next(48)
	Error(t, err)
	Nil(t, buffer)

	buffer, err = ringBuffer.Next(16)
	NoError(t, err)
	EqualValues(t, 16, len(buffer))

	buffer, err = ringBuffer.Next(-1)
	NoError(t, err)
	EqualValues(t, 16, len(buffer))
}

func TestMagicRingBufferWrite(t *testing.T) {
	ringBuffer := NewMagicBuffer(64)

	True(t, ringBuffer.IsEmpty(), "expect IsEmpty is true but got false")
	False(t, ringBuffer.IsFull(), "expect IsFull is false but got true")
	EqualValuesf(t, 0, ringBuffer.Buffered(), "expect len 0 bytes but got %d. r.w=%d, r.r=%d",
		ringBuffer.Buffered(), ringBuffer.w, ringBuffer.r)
	EqualValuesf(t, DefaultMagicBufferSize, ringBuffer.Available(),
		"expect free %d bytes but got %d. r.w=%d, r.r=%d",
		DefaultMagicBufferSize, ringBuffer.Available(), ringBuffer.w, ringBuffer.r)

	data := []byte(strings.Repeat("abcd", DefaultMagicBufferSize/16))
	bytesWritten, _ := ringBuffer.Write(data)
	EqualValuesf(t, DefaultMagicBufferSize/4, bytesWritten,
		"expect write %d bytes but got %d", DefaultMagicBufferSize/4, bytesWritten)
	EqualValuesf(t, DefaultMagicBufferSize/4, ringBuffer.Buffered(),
		"expect len %d bytes but got %d. r.w=%d, r.r=%d",
		DefaultMagicBufferSize/4, ringBuffer.Buffered(), ringBuffer.w, ringBuffer.r)
	EqualValuesf(t, (DefaultMagicBufferSize/4)*3, ringBuffer.Available(),
		"expect free %d bytes but got %d. r.w=%d, r.r=%d",
		(DefaultMagicBufferSize/4)*3, ringBuffer.Available(), ringBuffer.w, ringBuffer.r)
	EqualValuesf(t, data, ringBuffer.Bytes(),
		"expect repeated abcd but got %s. r.w=%d, r.r=%d", ringBuffer.Bytes(), ringBuffer.w, ringBuffer.r)

	False(t, ringBuffer.IsEmpty(), "expect IsEmpty is false but got true")
	False(t, ringBuffer.IsFull(), "expect IsFull is false but got true")

	data = []byte(strings.Repeat("abcd", (DefaultMagicBufferSize/16)*3))
	bytesWritten, _ = ringBuffer.Write(data)
	EqualValuesf(t, (DefaultMagicBufferSize/4)*3, bytesWritten,
		"expect write %d bytes but got %d", (DefaultMagicBufferSize/4)*3, bytesWritten)
	EqualValuesf(t, DefaultMagicBufferSize, ringBuffer.Buffered(),
		"expect len %d bytes but got %d. r.w=%d, r.r=%d",
		DefaultMagicBufferSize, ringBuffer.Buffered(), ringBuffer.w, ringBuffer.r)
	EqualValuesf(t, 0, ringBuffer.Available(),
		"expect free 0 bytes but got %d. r.w=%d, r.r=%d", ringBuffer.Available(), ringBuffer.w, ringBuffer.r)
	EqualValuesf(t, 0, ringBuffer.w, "expect r.w=0 but got %d. r.r=%d", ringBuffer.w, ringBuffer.r)
	EqualValuesf(t, []byte(strings.Repeat("abcd", DefaultMagicBufferSize/4)), ringBuffer.Bytes(),
		"expect repeated abcd but got %s. r.w=%d, r.r=%d", ringBuffer.Bytes(), ringBuffer.w, ringBuffer.r)

	False(t, ringBuffer.IsEmpty(), "expect IsEmpty is false but got true")
	True(t, ringBuffer.IsFull(), "expect IsFull is true but got false")
	True(t, ringBuffer.IsFull(), "expect IsFull is true but got false")

	bytesWritten, _ = ringBuffer.Write([]byte(strings.Repeat("abcd", DefaultMagicBufferSize/16)))
	EqualValuesf(t, DefaultMagicBufferSize/4, bytesWritten,
		"expect write %d bytes but got %d", DefaultMagicBufferSize/4, bytesWritten)
	size := ringBuffer.Cap()
	EqualValuesf(t, DefaultMagicBufferSize+DefaultMagicBufferSize/4, ringBuffer.Buffered(),
		"expect len %d bytes but got %d. r.w=%d, r.r=%d",
		DefaultMagicBufferSize+DefaultMagicBufferSize/4, ringBuffer.Buffered(), ringBuffer.w, ringBuffer.r)
	EqualValuesf(t, size-(DefaultMagicBufferSize+DefaultMagicBufferSize/4), ringBuffer.Available(),
		"expect free %d bytes but got %d. r.w=%d, r.r=%d",
		size-(DefaultMagicBufferSize+DefaultMagicBufferSize/4), ringBuffer.Available(), ringBuffer.w, ringBuffer.r)

	False(t, ringBuffer.IsEmpty(), "expect IsEmpty is false but got true")
	False(t, ringBuffer.IsFull(), "expect IsFull is false but got true")

	ringBuffer.Reset()
	bytesWritten, _ = ringBuffer.Write([]byte(strings.Repeat("abcd", DefaultMagicBufferSize/8)))
	EqualValuesf(t, DefaultMagicBufferSize/2, bytesWritten,
		"expect write %d bytes but got %d", DefaultMagicBufferSize/2, bytesWritten)
	EqualValuesf(t, DefaultMagicBufferSize/2, ringBuffer.Buffered(),
		"expect %d bytes but got %d. r.w=%d, r.r=%d",
		DefaultMagicBufferSize/2, ringBuffer.Buffered(), ringBuffer.w, ringBuffer.r)
	EqualValuesf(t, size-(DefaultMagicBufferSize/2), ringBuffer.Available(),
		"expect free %d bytes but got %d. r.w=%d, r.r=%d",
		size-(DefaultMagicBufferSize/2), ringBuffer.Available(), ringBuffer.w, ringBuffer.r)
	Greaterf(t, ringBuffer.w, 0, "expect r.w>=0 but got %d. r.r=%d", ringBuffer.w, ringBuffer.r)

	False(t, ringBuffer.IsEmpty(), "expect IsEmpty is false but got true")
	False(t, ringBuffer.IsFull(), "expect IsFull is false but got true")

	EqualValuesf(t, []byte(strings.Repeat("abcd", DefaultMagicBufferSize/8)), ringBuffer.Bytes(),
		"expect repeated abcd but got %s. r.w=%d, r.r=%d", ringBuffer.Bytes(), ringBuffer.w, ringBuffer.r)

	ringBuffer.Reset()
	size = ringBuffer.Cap()

	bytesWritten, _ = ringBuffer.Write([]byte(strings.Repeat("abcd", DefaultMagicBufferSize/32)))
	EqualValuesf(t, DefaultMagicBufferSize/8, bytesWritten,
		"expect write %d bytes but got %d", DefaultMagicBufferSize/8, bytesWritten)
	EqualValuesf(t, DefaultMagicBufferSize/8, ringBuffer.Buffered(),
		"expect len %d bytes but got %d. r.w=%d, r.r=%d",
		DefaultMagicBufferSize/8, ringBuffer.Buffered(), ringBuffer.w, ringBuffer.r)
	EqualValuesf(t, size-(DefaultMagicBufferSize/8), ringBuffer.Available(),
		"expect free %d bytes but got %d. r.w=%d, r.r=%d",
		size-(DefaultMagicBufferSize/8), ringBuffer.Available(), ringBuffer.w, ringBuffer.r)
	buf := make([]byte, DefaultMagicBufferSize/32)
	_, _ = ringBuffer.Read(buf)
	EqualValuesf(t, DefaultMagicBufferSize*3/32, ringBuffer.Buffered(),
		"expect len %d bytes but got %d. r.w=%d, r.r=%d",
		DefaultMagicBufferSize*3/32, ringBuffer.Buffered(), ringBuffer.w, ringBuffer.r)
	_, _ = ringBuffer.Write([]byte(strings.Repeat("abcd", DefaultMagicBufferSize/32)))

	EqualValuesf(t, []byte(strings.Repeat("abcd", DefaultMagicBufferSize*7/128)), ringBuffer.Bytes(),
		"expect repeated abcd ... but got %s. r.w=%d, r.r=%d", ringBuffer.Bytes(), ringBuffer.w, ringBuffer.r)
}

func TestZeroMagicRingBuffer(t *testing.T) {
	ringBuffer := NewMagicBuffer(0)
	buffer := ringBuffer.Peek(2)
	Empty(t, buffer, "buffer should be empty")
	buffer = ringBuffer.Peek(-1)
	Empty(t, buffer, "buffer should be empty")
	EqualValues(t, 0, ringBuffer.Buffered(), "expect length is 0")
	EqualValues(t, 0, ringBuffer.Available(), "expect free is 0")
	buf := []byte(strings.Repeat("1234", 12))
	_, _ = ringBuffer.Write(buf)

	EqualValuesf(t, DefaultMagicBufferSize, ringBuffer.Cap(),
		"expect rb.Cap()=%d, but got rb.Cap()=%d", DefaultMagicBufferSize, ringBuffer.Cap())
	Truef(t, ringBuffer.r == 0 && ringBuffer.w == 48 && ringBuffer.Size == DefaultMagicBufferSize,
		"expect rb.r=0, rb.w=48, rb.size=64, rb.mask=63, but got rb.r=%d, rb.w=%d, rb.size=%d",
		ringBuffer.r, ringBuffer.w, ringBuffer.Size)
	EqualValues(t, buf, ringBuffer.Bytes(), "expect it is equal")
	_ = ringBuffer.Discard(48)
	Truef(t, ringBuffer.IsEmpty() && ringBuffer.r == 0 && ringBuffer.w == 0,
		"expect rb is empty and rb.r=rb.w=0, but got rb.r=%d and rb.w=%d", ringBuffer.r, ringBuffer.w)
}

func TestMagicRingBufferGrow(t *testing.T) {
	ringBuffer := NewMagicBuffer(0)
	buffer := ringBuffer.Peek(2)
	Empty(t, buffer, "buffer should be empty")
	data := make([]byte, DefaultMagicBufferSize+1)
	bytesRead, err := rand.Read(data)
	NoError(t, err, "failed to generate random data")
	EqualValuesf(t, DefaultMagicBufferSize+1, bytesRead,
		"expect random data length is %d but got %d", DefaultMagicBufferSize+1, bytesRead)
	bytesWritten, err := ringBuffer.Write(data)
	NoError(t, err)
	EqualValues(t, DefaultMagicBufferSize+1, bytesWritten)
	EqualValues(t, 2*DefaultMagicBufferSize, ringBuffer.Cap())
	EqualValues(t, DefaultMagicBufferSize+1, ringBuffer.Buffered())
	EqualValues(t, DefaultMagicBufferSize-1, ringBuffer.Available())
	EqualValues(t, data, ringBuffer.Bytes())

	ringBuffer = NewMagicBuffer(DefaultMagicBufferSize)
	newData := make([]byte, DefaultMagicBufferSize+512)
	bytesRead, err = rand.Read(newData)
	NoError(t, err, "failed to generate random data")
	EqualValuesf(t, DefaultMagicBufferSize+512, bytesRead,
		"expect random data length is %d but got %d", DefaultMagicBufferSize+512, bytesRead)
	bytesWritten, err = ringBuffer.Write(newData)
	NoError(t, err)
	EqualValues(t, DefaultMagicBufferSize+512, bytesWritten)
	EqualValues(t, 2*DefaultMagicBufferSize, ringBuffer.Cap())
	EqualValues(t, DefaultMagicBufferSize+512, ringBuffer.Buffered())
	EqualValues(t, DefaultMagicBufferSize-512, ringBuffer.Available())
	EqualValues(t, newData, ringBuffer.Bytes())
}

func TestMagicRingBufferZeroes(t *testing.T) {
	ringBuffer := NewMagicBuffer(DefaultMagicBufferSize)

	True(t, ringBuffer.IsEmpty(), "expect IsEmpty is true but got false")
	False(t, ringBuffer.IsFull(), "expect isfull is false but got true")
	EqualValuesf(t, 0, ringBuffer.Buffered(),
		"expect len 0 bytes but got %d. r.w=%d, r.r=%d", ringBuffer.Buffered(), ringBuffer.w, ringBuffer.r)
	EqualValuesf(t, DefaultMagicBufferSize, ringBuffer.Available(),
		"expect free %d bytes but got %d. r.w=%d, r.r=%d",
		DefaultMagicBufferSize, ringBuffer.Available(), ringBuffer.w, ringBuffer.r)

	buffer := make([]byte, DefaultMagicBufferSize)
	n, err := rand.Read(buffer)
	NoError(t, err)
	Equal(t, DefaultMagicBufferSize, n)
	bytesWritten, err := ringBuffer.Write(buffer)
	NoErrorf(t, err, "expect err is nil but got %v", err)
	EqualValuesf(t, DefaultMagicBufferSize, bytesWritten,
		"expect read %d bytes but got %d", DefaultMagicBufferSize, bytesWritten)

	bytes := ringBuffer.Peek(-1)
	EqualValues(t, buffer, bytes)
	ringBuffer.Zeroes()
	EqualValues(t, make([]byte, DefaultMagicBufferSize*2), ringBuffer.vm.Buf)
}

func TestMagicRingBufferRead(t *testing.T) {
	ringBuffer := NewMagicBuffer(DefaultMagicBufferSize)

	True(t, ringBuffer.IsEmpty(), "expect IsEmpty is true but got false")
	False(t, ringBuffer.IsFull(), "expect isfull is false but got true")
	EqualValuesf(t, 0, ringBuffer.Buffered(),
		"expect len 0 bytes but got %d. r.w=%d, r.r=%d", ringBuffer.Buffered(), ringBuffer.w, ringBuffer.r)
	EqualValuesf(t, DefaultMagicBufferSize, ringBuffer.Available(),
		"expect free %d bytes but got %d. r.w=%d, r.r=%d",
		DefaultMagicBufferSize, ringBuffer.Available(), ringBuffer.w, ringBuffer.r)

	buffer := make([]byte, DefaultMagicBufferSize*2)
	bytesRead, err := ringBuffer.Read(buffer)
	ErrorIs(t, err, errors.ErrIsEmpty, "expect ErrIsEmpty but got nil")
	EqualValuesf(t, 0, bytesRead, "expect read 0 bytes but got %d", bytesRead)
	EqualValuesf(t, 0, ringBuffer.Buffered(), "expect len 0 bytes but got %d. r.w=%d, r.r=%d",
		ringBuffer.Buffered(), ringBuffer.w, ringBuffer.r)
	EqualValuesf(t, DefaultMagicBufferSize, ringBuffer.Available(),
		"expect free %d bytes but got %d. r.w=%d, r.r=%d",
		DefaultMagicBufferSize, ringBuffer.Available(), ringBuffer.w, ringBuffer.r)
	EqualValuesf(t, 0, ringBuffer.r, "expect r.r=0 but got %d. r.w=%d", ringBuffer.r, ringBuffer.w)

	_, _ = ringBuffer.Write([]byte(strings.Repeat("abcd", DefaultMagicBufferSize/16)))
	bytesRead, err = ringBuffer.Read(buffer)
	NoErrorf(t, err, "read failed: %v", err)
	EqualValuesf(t, DefaultMagicBufferSize/4, bytesRead,
		"expect read %d bytes but got %d", DefaultMagicBufferSize/4, bytesRead)
	EqualValuesf(t, 0, ringBuffer.Buffered(),
		"expect len 0 bytes but got %d. r.w=%d, r.r=%d", ringBuffer.Buffered(), ringBuffer.w, ringBuffer.r)
	EqualValuesf(t, DefaultMagicBufferSize, ringBuffer.Available(),
		"expect free %d bytes but got %d. r.w=%d, r.r=%d",
		DefaultMagicBufferSize, ringBuffer.Available(), ringBuffer.w, ringBuffer.r)
	EqualValuesf(t, 0, ringBuffer.r, "expect r.r=0 but got %d. r.w=%d", ringBuffer.r, ringBuffer.w)

	_, _ = ringBuffer.Write([]byte(strings.Repeat("abcde", DefaultMagicBufferSize/4)))
	bytesRead, err = ringBuffer.Read(buffer)
	NoErrorf(t, err, "read failed: %v", err)
	EqualValuesf(t, DefaultMagicBufferSize+DefaultMagicBufferSize/4, bytesRead,
		"expect read %d bytes but got %d", DefaultMagicBufferSize+DefaultMagicBufferSize/4, bytesRead)
	EqualValuesf(t, 0, ringBuffer.Buffered(),
		"expect len 0 bytes but got %d. r.w=%d, r.r=%d", ringBuffer.Buffered(), ringBuffer.w, ringBuffer.r)
	EqualValuesf(t, DefaultMagicBufferSize*2, ringBuffer.Available(),
		"expect free 128 bytes but got %d. r.w=%d, r.r=%d", ringBuffer.Available(), ringBuffer.w, ringBuffer.r)
	EqualValuesf(t, 0, ringBuffer.r, "expect r.r=0 but got %d. r.w=%d", ringBuffer.r, ringBuffer.w)

	ringBuffer.Reset()
	_, _ = ringBuffer.Write([]byte(strings.Repeat("12345678", DefaultMagicBufferSize/4)))
	True(t, ringBuffer.IsFull(), "ring buffer should be full")
	EqualValuesf(t, 0, ringBuffer.Available(),
		"expect free 0 bytes but got %d. r.w=%d, r.r=%d", ringBuffer.Available(), ringBuffer.w, ringBuffer.r)
	EqualValuesf(t, 0, ringBuffer.w, "expect r.2=0 but got %d. r.r=%d", ringBuffer.w, ringBuffer.r)
	peekBuffer := ringBuffer.Peek(DefaultMagicBufferSize)
	Truef(t, len(peekBuffer) == DefaultMagicBufferSize,
		"expect len(peekBuffer)=%d, yet len(peekBuffer)=%d", DefaultMagicBufferSize, len(peekBuffer))
	EqualValuesf(t, 0, ringBuffer.r, "expect r.r=0 but got %d", ringBuffer.r)
	EqualValues(t, []byte(strings.Repeat("12345678", DefaultMagicBufferSize/8)), peekBuffer)
	_ = ringBuffer.Discard(DefaultMagicBufferSize)
	EqualValuesf(t, DefaultMagicBufferSize, ringBuffer.r,
		"expect r.r=%d but got %d", DefaultMagicBufferSize, ringBuffer.r)
	_, _ = ringBuffer.Write([]byte(strings.Repeat("1234", DefaultMagicBufferSize/16)))
	EqualValuesf(t, DefaultMagicBufferSize/4, ringBuffer.w,
		"expect r.w=%d but got %d", DefaultMagicBufferSize/4, ringBuffer.w)
	peekBuffer = ringBuffer.Peek(DefaultMagicBufferSize * 2)
	Truef(t, len(peekBuffer) == DefaultMagicBufferSize+DefaultMagicBufferSize/4,
		"expect len(buffer)=%d, yet len(buffer)=%d",
		DefaultMagicBufferSize+DefaultMagicBufferSize/4, len(peekBuffer))
	EqualValues(t, append(
		[]byte(strings.Repeat("12345678", DefaultMagicBufferSize/8)),
		[]byte(strings.Repeat("1234", DefaultMagicBufferSize/16))...,
	), peekBuffer)
	peekBuffer = ringBuffer.Peek(-1)
	Truef(t, len(peekBuffer) == DefaultMagicBufferSize+DefaultMagicBufferSize/4,
		"expect len(peekBuffer)=%d, yet len(peekBuffer)=%d",
		DefaultMagicBufferSize+DefaultMagicBufferSize/4, len(peekBuffer))
	EqualValues(t, append(
		[]byte(strings.Repeat("12345678", DefaultMagicBufferSize/8)),
		[]byte(strings.Repeat("1234", DefaultMagicBufferSize/16))...,
	), peekBuffer)
	_ = ringBuffer.Discard(DefaultMagicBufferSize)
	_ = ringBuffer.Discard(DefaultMagicBufferSize / 4)
	True(t, ringBuffer.IsEmpty(), "should be empty")
}

func TestMagicRingBufferByteInterface(t *testing.T) {
	ringBuffer := NewMagicBuffer(0)

	bytesWritten, _ := ringBuffer.Write([]byte(strings.Repeat("a", DefaultMagicBufferSize-2)))
	EqualValuesf(t, DefaultMagicBufferSize-2, bytesWritten,
		"expect write %d bytes but got %d", DefaultMagicBufferSize-2, bytesWritten)

	_ = ringBuffer.WriteByte('a')
	EqualValuesf(t, DefaultMagicBufferSize-1, ringBuffer.Buffered(),
		"expect len %d bytes but got %d. r.w=%d, r.r=%d",
		DefaultMagicBufferSize-1, ringBuffer.Buffered(), ringBuffer.w, ringBuffer.r)
	EqualValuesf(t, 1, ringBuffer.Available(),
		"expect free 1 byte but got %d. r.w=%d, r.r=%d", ringBuffer.Available(), ringBuffer.w, ringBuffer.r)
	EqualValuesf(t, []byte(strings.Repeat("a", DefaultMagicBufferSize-1)), ringBuffer.Bytes(),
		"expect a but got %s. r.w=%d, r.r=%d", ringBuffer.Bytes(), ringBuffer.w, ringBuffer.r)

	Falsef(t, ringBuffer.IsEmpty(), "expect IsEmpty is false but got true")
	False(t, ringBuffer.IsFull(), "expect IsFull is false but got true")

	_ = ringBuffer.WriteByte('b')
	EqualValuesf(t, DefaultMagicBufferSize, ringBuffer.Buffered(),
		"expect len %d bytes but got %d. r.w=%d, r.r=%d",
		DefaultMagicBufferSize, ringBuffer.Buffered(), ringBuffer.w, ringBuffer.r)
	EqualValuesf(t, 0, ringBuffer.Available(),
		"expect free 0 byte but got %d. r.w=%d, r.r=%d", ringBuffer.Available(), ringBuffer.w, ringBuffer.r)
	EqualValuesf(t, append([]byte(strings.Repeat("a", DefaultMagicBufferSize-1)),
		[]byte{'b'}...), ringBuffer.Bytes(),
		"expect a but got %s. r.w=%d, r.r=%d", ringBuffer.Bytes(), ringBuffer.w, ringBuffer.r)

	False(t, ringBuffer.IsEmpty(), "expect IsEmpty is false but got true")
	True(t, ringBuffer.IsFull(), "expect IsFull is true but got false")

	_ = ringBuffer.WriteByte('c')
	EqualValuesf(t, DefaultMagicBufferSize+1, ringBuffer.Buffered(),
		"expect len %d bytes but got %d. r.w=%d, r.r=%d",
		DefaultMagicBufferSize+1, ringBuffer.Buffered(), ringBuffer.w, ringBuffer.r)
	EqualValuesf(t, DefaultMagicBufferSize-1, ringBuffer.Available(),
		"expect free %d bytes but got %d. r.w=%d, r.r=%d",
		DefaultMagicBufferSize-1, ringBuffer.Available(), ringBuffer.w, ringBuffer.r)
	EqualValuesf(t, append([]byte(strings.Repeat("a", DefaultMagicBufferSize-1)),
		[]byte{'b', 'c'}...), ringBuffer.Bytes(),
		"expect a but got %s. r.w=%d, r.r=%d", ringBuffer.Bytes(), ringBuffer.w, ringBuffer.r)

	False(t, ringBuffer.IsEmpty(), "expect IsEmpty is false but got true")
	False(t, ringBuffer.IsFull(), "expect IsFull is false but got true")

	buf := make([]byte, DefaultMagicBufferSize-2)
	bytesWritten, err := ringBuffer.Read(buf)
	NoErrorf(t, err, "Read failed: %v", err)
	EqualValuesf(t, DefaultMagicBufferSize-2, bytesWritten, "expect read %d bytes but got %d",
		DefaultMagicBufferSize, bytesWritten)

	nextByte, err := ringBuffer.ReadByte()
	NoErrorf(t, err, "ReadByte failed: %v", err)
	EqualValuesf(t, 'a', nextByte, "expect a but got %c. r.w=%d, r.r=%d", nextByte, ringBuffer.w, ringBuffer.r)
	EqualValuesf(t, 2, ringBuffer.Buffered(),
		"expect len 2 byte but got %d. r.w=%d, r.r=%d", ringBuffer.Buffered(), ringBuffer.w, ringBuffer.r)
	EqualValuesf(t, DefaultMagicBufferSize*2-2, ringBuffer.Available(),
		"expect free %d bytes but got %d. r.w=%d, r.r=%d",
		DefaultMagicBufferSize*2-2, ringBuffer.Available(), ringBuffer.w, ringBuffer.r)
	EqualValuesf(t, []byte{'b', 'c'}, ringBuffer.Bytes(),
		"expect a but got %s. r.w=%d, r.r=%d", ringBuffer.Bytes(), ringBuffer.w, ringBuffer.r)
	False(t, ringBuffer.IsEmpty(), "expect IsEmpty is false but got true")
	False(t, ringBuffer.IsFull(), "expect IsFull is false but got true")

	nextByte, _ = ringBuffer.ReadByte()
	EqualValuesf(t, 'b', nextByte, "expect b but got %c. r.w=%d, r.r=%d", nextByte, ringBuffer.w, ringBuffer.r)
	EqualValuesf(t, 1, ringBuffer.Buffered(),
		"expect len 1 byte but got %d. r.w=%d, r.r=%d", ringBuffer.Buffered(), ringBuffer.w, ringBuffer.r)
	EqualValuesf(t, DefaultMagicBufferSize*2-1, ringBuffer.Available(),
		"expect free %d bytes but got %d. r.w=%d, r.r=%d",
		DefaultMagicBufferSize*2-1, ringBuffer.Available(), ringBuffer.w, ringBuffer.r)
	False(t, ringBuffer.IsEmpty(), "expect IsEmpty is false but got true")
	False(t, ringBuffer.IsFull(), "expect IsFull is false but got true")

	_, _ = ringBuffer.ReadByte()
	EqualValuesf(t, 0, ringBuffer.Buffered(),
		"expect len 0 byte but got %d. r.w=%d, r.r=%d", ringBuffer.Buffered(), ringBuffer.w, ringBuffer.r)
	EqualValuesf(t, DefaultMagicBufferSize*2, ringBuffer.Available(),
		"expect free %d bytes but got %d. r.w=%d, r.r=%d",
		DefaultMagicBufferSize*2, ringBuffer.Available(), ringBuffer.w, ringBuffer.r)
	True(t, ringBuffer.IsEmpty(), "expect IsEmpty is true but got false")
	False(t, ringBuffer.IsFull(), "expect IsFull is false but got true")
}

func TestMagicRingBufferReadFrom(t *testing.T) {
	ringBuffer := NewMagicBuffer(0)
	dataLen := 4 * DefaultMagicBufferSize
	data := make([]byte, dataLen)
	_, err := rand.Read(data)
	NoError(t, err)
	reader := bytes.NewReader(data)
	bytesReadFrom, err := ringBuffer.ReadFrom(reader)
	NoError(t, err)
	False(t, ringBuffer.IsEmpty())
	EqualValuesf(t, dataLen, bytesReadFrom, "ringbuffer should read %d bytes, but got %d", dataLen, bytesReadFrom)
	EqualValuesf(t, dataLen, ringBuffer.Buffered(),
		"ringbuffer should have %d bytes, but got %d", dataLen, ringBuffer.Buffered())
	buf := ringBuffer.Peek(-1)
	EqualValues(t, data, buf)
	buf = make([]byte, dataLen)

	var bytesRead int
	bytesRead, err = ringBuffer.Read(buf)
	NoError(t, err)
	EqualValuesf(t, dataLen, bytesRead, "ringbuffer should read %d bytes, but got %d", dataLen, bytesRead)
	EqualValues(t, data, buf)
	Truef(t, ringBuffer.IsEmpty(), "ringbuffer should be empty, but it isn't")
	Zerof(t, ringBuffer.Buffered(), "ringbuffer should be empty, but still have %d bytes", ringBuffer.Buffered())

	ringBuffer = NewMagicBuffer(0)
	prefixLen := DefaultMagicBufferSize / 2
	prefix := make([]byte, prefixLen)
	_, err = rand.Read(prefix)
	NoError(t, err)
	offset := prefixLen / 2

	dataLen = DefaultMagicBufferSize - offset
	data = make([]byte, dataLen)
	_, err = rand.Read(data)
	NoError(t, err)
	reader.Reset(data)
	bytesWritten, err := ringBuffer.Write(prefix)
	NoError(t, err)
	EqualValuesf(t, prefixLen, bytesWritten,
		"ringbuffer should write %d bytes, but got %d", prefixLen, bytesWritten)
	ringBuffer.AdvanceRead(offset)
	bytesFromRead, err := ringBuffer.ReadFrom(reader)
	NoError(t, err)
	EqualValuesf(t, dataLen, bytesFromRead, "ringbuffer should read %d bytes, but got %d", dataLen, bytesFromRead)
	EqualValuesf(t, prefixLen+dataLen-prefixLen/2, ringBuffer.Buffered(),
		"ringbuffer should have %d bytes, but got %d", prefixLen+dataLen-prefixLen/2, ringBuffer.Buffered())

	peekBuffer := ringBuffer.Peek(offset)
	EqualValues(t, prefix[offset:], peekBuffer)
	_ = ringBuffer.Discard(offset)
	EqualValuesf(t, dataLen, ringBuffer.Buffered(),
		"ringbuffer should have %d bytes, but got %d", dataLen, ringBuffer.Buffered())
	peekBuffer = ringBuffer.Peek(-1)
	EqualValues(t, data, peekBuffer)
	_ = ringBuffer.Discard(dataLen)
	Truef(t, ringBuffer.IsEmpty(), "ringbuffer should be empty, but it isn't")
	Zerof(t, ringBuffer.Buffered(), "ringbuffer should be empty, but still have %d bytes", ringBuffer.Buffered())

	ringBuffer = NewMagicBuffer(0)
	prefixLen = DefaultMagicBufferSize
	prefix = make([]byte, prefixLen)
	_, err = rand.Read(prefix)
	NoError(t, err)
	_, err = rand.Read(data)
	NoError(t, err)
	reader.Reset(data)
	bytesWritten, err = ringBuffer.Write(prefix)
	NoError(t, err)
	EqualValuesf(t, prefixLen, bytesWritten,
		"ringbuffer should read %d bytes, but got %d", prefixLen, bytesWritten)

	const partLen = 1024
	peekBuffer = ringBuffer.Peek(partLen)
	EqualValues(t, prefix[:partLen], peekBuffer)
	_ = ringBuffer.Discard(partLen)
	bytesReadFrom, err = ringBuffer.ReadFrom(reader)
	NoError(t, err)
	EqualValuesf(t, dataLen, bytesReadFrom, "ringbuffer should read %d bytes, but got %d", dataLen, bytesReadFrom)
	peekBuffer = ringBuffer.Peek(-1)
	EqualValues(t, append(prefix[partLen:], data...), peekBuffer)
	_ = ringBuffer.Discard(prefixLen + dataLen - partLen)
	Truef(t, ringBuffer.IsEmpty(), "ringbuffer should be empty, but it isn't")
	Zerof(t, ringBuffer.Buffered(), "ringbuffer should be empty, but still have %d bytes", ringBuffer.Buffered())
}

func TestMagicRingBufferWriteTo(t *testing.T) {
	ringBuffer := NewMagicBuffer(5 * 1024)

	const dataLen = 4 * 1024
	data := make([]byte, dataLen)
	_, err := rand.Read(data)
	NoError(t, err)
	bytesWritten, err := ringBuffer.Write(data)
	NoError(t, err)
	EqualValuesf(t, dataLen, bytesWritten, "ringbuffer should write %d bytes, but got %d", dataLen, bytesWritten)
	buf := bytes.NewBuffer(nil)

	var bytesWrittenTo int64
	bytesWrittenTo, err = ringBuffer.WriteTo(buf)
	NoError(t, err)
	True(t, ringBuffer.IsEmpty())
	EqualValuesf(t, dataLen, bytesWrittenTo,
		"ringbuffer should write %d bytes, but got %d", dataLen, bytesWrittenTo)
	EqualValues(t, data, buf.Bytes())

	buf.Reset()
	_, err = rand.Read(data)
	NoError(t, err)
	ringBuffer = NewMagicBuffer(dataLen)
	bytesWritten, err = ringBuffer.Write(data)
	NoError(t, err)
	EqualValuesf(t, dataLen, bytesWritten, "ringbuffer should write %d bytes, but got %d", dataLen, bytesWritten)
	Truef(t, ringBuffer.IsFull(), "ringbuffer should be full, but it isn't")
	bytesWrittenTo, err = ringBuffer.WriteTo(buf)
	NoError(t, err)
	True(t, ringBuffer.IsEmpty())
	EqualValuesf(t, dataLen, bytesWrittenTo,
		"ringbuffer should write %d bytes, but got %d", dataLen, bytesWrittenTo)
	EqualValues(t, data, buf.Bytes())

	buf.Reset()
	ringBuffer.Reset()
	_, err = rand.Read(data)
	NoError(t, err)
	bytesWritten, err = ringBuffer.Write(data)
	NoError(t, err)
	EqualValuesf(t, dataLen, bytesWritten, "ringbuffer should write %d bytes, but got %d", dataLen, bytesWritten)
	Truef(t, ringBuffer.IsFull(), "ringbuffer should be full, but it isn't")

	const partLen = 1024
	peekBuffer := ringBuffer.Peek(partLen)
	EqualValues(t, data[:partLen], peekBuffer)
	_ = ringBuffer.Discard(partLen)
	partData := make([]byte, partLen/2)
	_, err = rand.Read(partData)
	NoError(t, err)
	bytesWritten, err = ringBuffer.Write(partData)
	NoError(t, err)
	EqualValuesf(t, partLen/2, bytesWritten, "ringbuffer should write %d bytes, but got %d", dataLen, bytesWritten)
	EqualValues(t, partLen/2, ringBuffer.Available())
	bytesWrittenTo, err = ringBuffer.WriteTo(buf)
	NoError(t, err)
	EqualValuesf(t, dataLen-partLen/2, bytesWrittenTo,
		"ringbuffer should write %d bytes, but got %d", dataLen-partLen/2, bytesWrittenTo)
	EqualValues(t, append(data[partLen:], partData...), buf.Bytes())
	True(t, ringBuffer.IsEmpty())
}
