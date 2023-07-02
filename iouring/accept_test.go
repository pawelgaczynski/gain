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

func TestAccept(t *testing.T) {
	ring, err := iouring.CreateRing(16)
	Nil(t, err)

	defer func() {
		_ = ring.QueueExit()
	}()

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
	err = syscall.Listen(socketFd, 1)
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

	clientConnChan := make(chan net.Conn)
	go func() {
		conn, dialErr := net.DialTimeout(gnet.TCP, fmt.Sprintf("127.0.0.1:%d", testPort), time.Second)
		Nil(t, dialErr)
		clientConnChan <- conn
	}()

	defer func() {
		conn := <-clientConnChan
		if tcpConn, ok := conn.(*net.TCPConn); ok {
			lErr := tcpConn.SetLinger(0)
			Nil(t, lErr)
		}
	}()

	cqes := make([]*iouring.CompletionQueueEvent, 128)

	for {
		numberOfCQEs := ring.PeekBatchCQE(cqes)
		for i := 0; i < numberOfCQEs; i++ {
			cqe := cqes[i]
			Equal(t, uint64(socketFd), cqe.UserData())
			Greater(t, cqe.Res(), int32(0))
			err = syscall.Shutdown(int(cqe.Res()), syscall.SHUT_RDWR)
			Nil(t, err)
		}

		if numberOfCQEs > 0 {
			ring.CQAdvance(uint32(numberOfCQEs))
			Nil(t, err)

			break
		}
	}
}
