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
	. "github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

const (
	accept = iota
	recv
	send
)

type connection struct {
	fd     uint64
	buffer []byte
	state  int
}

func loop(t *testing.T, ring *iouring.Ring, socketFd int, connection *connection) bool {
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
	case accept:
		Equal(t, uint64(socketFd), cqe.UserData())
		Greater(t, cqe.Res(), int32(0))
		connection.fd = uint64(cqe.Res())
		entry.PrepareRecv(
			int(connection.fd), uintptr(unsafe.Pointer(&connection.buffer[0])), uint32(len(connection.buffer)), 0)
		entry.UserData = connection.fd

		connection.state = recv
	case recv:
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

		connection.state = send
	case send:
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

func TestRecvSend(t *testing.T) {
	ring, err := iouring.CreateRing()
	Nil(t, err)
	defer ring.Close()

	socketFd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	Nil(t, err)
	err = syscall.SetsockoptInt(socketFd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	Nil(t, err)
	err = syscall.Bind(socketFd, &syscall.SockaddrInet4{
		Port: testPort,
	})
	Nil(t, err)
	err = syscall.SetNonblock(socketFd, false)
	Nil(t, err)
	err = syscall.Listen(socketFd, 128)
	Nil(t, err)
	defer func() {
		err := syscall.Close(socketFd)
		Nil(t, err)
	}()

	entry, err := ring.GetSQE()
	Nil(t, err)
	var clientLen = new(uint32)
	clientAddr := &unix.RawSockaddrAny{}
	*clientLen = unix.SizeofSockaddrAny
	clientAddrPointer := uintptr(unsafe.Pointer(clientAddr))
	clientLenPointer := uint64(uintptr(unsafe.Pointer(clientLen)))

	entry.PrepareAccept(int(uintptr(socketFd)), clientAddrPointer, clientLenPointer, 0)
	entry.UserData = uint64(socketFd)
	cqeNr, err := ring.Submit()
	Nil(t, err)
	Equal(t, uint(1), cqeNr)

	connection := &connection{state: accept, buffer: make([]byte, 64)}

	clientConnChan := make(chan net.Conn)
	go func() {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", testPort), time.Second)
		Nil(t, err)
		NotNil(t, conn)
		n, err := conn.Write([]byte("testdata1234567890"))
		Nil(t, err)
		Equal(t, 18, n)
		var buffer [22]byte
		n, err = conn.Read(buffer[:])
		Nil(t, err)
		Equal(t, 22, n)
		Equal(t, "responsedata0123456789", string(buffer[:]))
		clientConnChan <- conn
	}()
	defer func() {
		conn := <-clientConnChan
		err = conn.(*net.TCPConn).SetLinger(0)
		Nil(t, err)
	}()
	for {
		if loop(t, ring, socketFd, connection) {
			break
		}
	}
}
