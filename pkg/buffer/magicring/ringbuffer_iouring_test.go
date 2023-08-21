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

package magicring

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"syscall"
	"testing"
	"time"
	"unsafe"

	gainNet "github.com/pawelgaczynski/gain/pkg/net"
	"github.com/pawelgaczynski/giouring"
	. "github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

const (
	accept = iota
	recv
	send
)

type conn struct {
	fd             uint64
	inboundBuffer  *RingBuffer
	outboundBuffer *RingBuffer
	state          int
}

func loop(t *testing.T, ring *giouring.Ring, socketFd int, connection *conn, testCase *testCase) bool {
	t.Helper()

	cqe, err := ring.WaitCQE()
	if errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EINTR) ||
		errors.Is(err, syscall.ETIME) {
		return false
	}

	Nil(t, err)
	entry := ring.GetSQE()
	NotNil(t, entry)
	ring.CQESeen(cqe)

	switch connection.state {
	case accept:
		Equal(t, uint64(socketFd), cqe.UserData)
		Greater(t, cqe.Res, int32(0))
		connection.fd = uint64(cqe.Res)
		entry.PrepareRecv(
			int(connection.fd),
			uintptr(connection.inboundBuffer.WriteAddress()),
			uint32(connection.inboundBuffer.Available()),
			0)
		entry.UserData = connection.fd
		connection.state = recv

	case recv:
		var data []byte
		if testCase.recvIdx == 0 {
			data = testCase.halfLenData
		} else {
			data = testCase.wholeLenData
		}
		testCase.recvIdx++

		Equal(t, connection.fd, cqe.UserData)
		Equal(t, int32(len(data)), cqe.Res)

		connection.inboundBuffer.AdvanceWrite(int(cqe.Res))
		readBuf := make([]byte, DefaultMagicBufferSize)

		var bytesRead int
		bytesRead, err = connection.inboundBuffer.Read(readBuf)
		Nil(t, err)
		Equal(t, len(data), bytesRead)
		Equal(t, data, readBuf[:cqe.Res])

		var bytesWritten int
		bytesWritten, err = connection.outboundBuffer.Write(data)
		Nil(t, err)
		Equal(t, len(data), bytesWritten)

		entry.PrepareSend(
			int(connection.fd),
			uintptr(connection.outboundBuffer.ReadAddress()),
			uint32(connection.outboundBuffer.Buffered()),
			0)
		entry.UserData = connection.fd
		connection.state = send

	case send:
		var res int32
		if testCase.sendIdx == 0 {
			res = int32(DefaultMagicBufferSize / 2)
		} else {
			res = int32(DefaultMagicBufferSize)
		}

		Equal(t, connection.fd, cqe.UserData)
		Equal(t, res, cqe.Res)

		connection.outboundBuffer.AdvanceRead(int(cqe.Res))

		if testCase.sendIdx == 0 {
			entry.PrepareRecv(
				int(connection.fd),
				uintptr(connection.inboundBuffer.WriteAddress()),
				uint32(connection.inboundBuffer.Available()),
				0)
			entry.UserData = connection.fd
			connection.state = recv
			testCase.sendIdx++
		} else {
			err = syscall.Shutdown(int(connection.fd), syscall.SHUT_RDWR)
			Nil(t, err)

			return true
		}
	}
	cqeNr, err := ring.Submit()
	Nil(t, err)
	Equal(t, uint(1), cqeNr)

	return false
}

type testCase struct {
	halfLenData  []byte
	wholeLenData []byte
	recvIdx      int
	sendIdx      int
}

func TestMagicRingRecvSend(t *testing.T) {
	ring, err := giouring.CreateRing(16)
	Nil(t, err)

	defer ring.QueueExit()

	socketFd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	Nil(t, err)
	err = syscall.SetsockoptInt(socketFd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	Nil(t, err)
	err = syscall.Bind(socketFd, &syscall.SockaddrInet4{
		Port: 9876,
	})
	Nil(t, err)
	err = syscall.SetNonblock(socketFd, false)
	Nil(t, err)
	err = syscall.Listen(socketFd, 128)
	Nil(t, err)

	defer func() {
		closeErr := syscall.Close(socketFd)
		Nil(t, closeErr)
	}()

	entry := ring.GetSQE()
	NotNil(t, entry)
	clientLen := new(uint32)
	clientAddr := &unix.RawSockaddrAny{}
	*clientLen = unix.SizeofSockaddrAny
	clientAddrPointer := uintptr(unsafe.Pointer(clientAddr))
	clientLenPointer := uint64(uintptr(unsafe.Pointer(clientLen)))
	entry.PrepareAccept(int(uintptr(socketFd)), clientAddrPointer, clientLenPointer, 0)
	entry.UserData = uint64(socketFd)
	cqeNr, err := ring.Submit()
	Nil(t, err)
	Equal(t, uint(1), cqeNr)

	wholeLenData := make([]byte, DefaultMagicBufferSize)
	halfLenData := make([]byte, DefaultMagicBufferSize/2)
	bytesRead, err := rand.Read(wholeLenData)
	Nil(t, err)
	Equal(t, DefaultMagicBufferSize, bytesRead)
	bytesRead, err = rand.Read(halfLenData)
	Nil(t, err)
	Equal(t, DefaultMagicBufferSize/2, bytesRead)
	connection := &conn{
		state:          accept,
		inboundBuffer:  NewMagicBuffer(DefaultMagicBufferSize),
		outboundBuffer: NewMagicBuffer(DefaultMagicBufferSize),
	}

	clientConnChan := make(chan net.Conn)
	go func() {
		conn, cErr := net.DialTimeout(gainNet.TCP, fmt.Sprintf("127.0.0.1:%d", 9876), time.Second)
		Nil(t, cErr)
		NotNil(t, conn)

		var bytesWritten int
		bytesWritten, cErr = conn.Write(halfLenData)
		Nil(t, cErr)
		Equal(t, DefaultMagicBufferSize/2, bytesWritten)
		buffer := make([]byte, DefaultMagicBufferSize)
		bytesWritten, cErr = conn.Read(buffer)
		Nil(t, cErr)
		Equal(t, len(halfLenData), bytesWritten)
		Equal(t, halfLenData, buffer[:DefaultMagicBufferSize/2])
		bytesWritten, cErr = conn.Write(wholeLenData)
		Nil(t, cErr)
		Equal(t, DefaultMagicBufferSize, bytesWritten)
		bytesWritten, cErr = conn.Read(buffer)
		Nil(t, cErr)
		Equal(t, len(wholeLenData), bytesWritten)
		Equal(t, wholeLenData, buffer[:DefaultMagicBufferSize])

		clientConnChan <- conn
	}()

	defer func() {
		conn := <-clientConnChan
		if tcpConn, ok := conn.(*net.TCPConn); ok {
			lErr := tcpConn.SetLinger(0)
			Nil(t, lErr)
		}
	}()

	testCase := &testCase{
		halfLenData:  halfLenData,
		wholeLenData: wholeLenData,
	}

	for {
		if loop(t, ring, socketFd, connection, testCase) {
			break
		}
	}
}
