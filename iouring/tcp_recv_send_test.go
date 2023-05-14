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

package iouring_test

import (
	"errors"
	"fmt"
	"net"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/pawelgaczynski/gain/iouring"
	gnet "github.com/pawelgaczynski/gain/pkg/net"
	. "github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

const (
	tcpAccept = iota
	tcpRecv
	tcpSend
)

type tcpConnection struct {
	fd     uint64
	buffer []byte
	state  int
}

func tcpLoop(t *testing.T, ring *iouring.Ring, socketFd int, connection *tcpConnection) bool {
	t.Helper()

	cqe, err := ring.WaitCQE()
	if errors.Is(err, iouring.ErrAgain) || errors.Is(err, iouring.ErrInterrupredSyscall) ||
		errors.Is(err, iouring.ErrTimerExpired) {
		return false
	}

	Nil(t, err)
	entry, err := ring.GetSQE()
	Nil(t, err)
	ring.CQESeen(cqe)

	switch connection.state {
	case tcpAccept:
		Equal(t, uint64(socketFd), cqe.UserData())
		Greater(t, cqe.Res(), int32(0))
		connection.fd = uint64(cqe.Res())
		entry.PrepareRecv(
			int(connection.fd), uintptr(unsafe.Pointer(&connection.buffer[0])), uint32(len(connection.buffer)), 0)
		entry.UserData = connection.fd
		connection.state = tcpRecv

	case tcpRecv:
		Equal(t, connection.fd, cqe.UserData())
		Equal(t, cqe.Res(), int32(18))
		Equal(t, "testdata1234567890", string(connection.buffer[:18]))
		connection.buffer = connection.buffer[:0]
		data := []byte("responsedata0123456789")
		copied := copy(connection.buffer[:len(data)], data)
		Equal(t, 22, copied)
		buffer := connection.buffer[:len(data)]
		entry.PrepareSend(
			int(connection.fd), uintptr(unsafe.Pointer(&buffer[0])), uint32(len(buffer)), 0)
		entry.UserData = connection.fd
		connection.state = tcpSend

	case tcpSend:
		Equal(t, connection.fd, cqe.UserData())
		Greater(t, cqe.Res(), int32(0))
		err = syscall.Shutdown(int(connection.fd), syscall.SHUT_RDWR)
		Nil(t, err)

		return true
	}
	cqeNr, err := ring.Submit()
	Nil(t, err)
	Equal(t, uint(1), cqeNr)

	return false
}

func TestTCPRecvSend(t *testing.T) {
	ring, err := iouring.CreateRing()
	Nil(t, err)

	defer ring.Close()

	socketFd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	Nil(t, err)
	err = syscall.SetsockoptInt(socketFd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	Nil(t, err)
	testPort := getTestPort()
	err = syscall.Bind(socketFd, &syscall.SockaddrInet4{
		Port: testPort,
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

	entry, err := ring.GetSQE()
	Nil(t, err)
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

	connection := &tcpConnection{state: tcpAccept, buffer: make([]byte, 64)}

	clientConnChan := make(chan net.Conn)
	go func() {
		conn, cErr := net.DialTimeout(gnet.TCP, fmt.Sprintf("127.0.0.1:%d", testPort), time.Second)
		Nil(t, cErr)
		NotNil(t, conn)
		bytesWritten, cErr := conn.Write([]byte("testdata1234567890"))
		Nil(t, cErr)
		Equal(t, 18, bytesWritten)

		var buffer [22]byte
		bytesWritten, cErr = conn.Read(buffer[:])
		Nil(t, cErr)
		Equal(t, 22, bytesWritten)
		Equal(t, "responsedata0123456789", string(buffer[:]))
		clientConnChan <- conn
	}()

	defer func() {
		conn := <-clientConnChan
		if tcpConn, ok := conn.(*net.TCPConn); ok {
			lErr := tcpConn.SetLinger(0)
			Nil(t, lErr)
		}
	}()

	for {
		if tcpLoop(t, ring, socketFd, connection) {
			break
		}
	}
}
