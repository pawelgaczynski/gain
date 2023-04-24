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

package gain_test

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pawelgaczynski/gain"
	gainErrors "github.com/pawelgaczynski/gain/pkg/errors"
	gainNet "github.com/pawelgaczynski/gain/pkg/net"
	"github.com/rs/zerolog"
	. "github.com/stretchr/testify/require"
)

type testServerConfig struct {
	protocol              string
	numberOfClients       int
	numberOfWorkers       int
	cpuAffinity           bool
	asyncHandler          bool
	goroutinePool         bool
	waitForDialAllClients bool
	afterDial             afterDialCallback
	writesCount           int

	readHandler onReadCallback
}

var defaultTestOnReadCallback = func(c gain.Conn) {
	buffer := make([]byte, 128)

	_, err := c.Read(buffer)
	if err != nil {
		if errors.Is(err, gainErrors.ErrIsEmpty) {
			return
		}

		log.Panic(err)
	}

	if string(buffer[0:6]) != "cindex" {
		log.Panic(fmt.Errorf("unexpected data: %s", string(buffer[0:6])))
	}

	_, err = c.Write(append(buffer[0:10], []byte("TESTpayload12345")...))
	if err != nil {
		log.Panic(err)
	}
}

type testServerHandler struct {
	onStartCallback onStartCallback
	onOpenCallback  onOpenCallback
	onReadCallback  onReadCallback
	onWriteCallback onWriteCallback
	onCloseCallback onCloseCallback

	startedWg *sync.WaitGroup
}

func (h *testServerHandler) OnStart(server gain.Server) {
	h.startedWg.Done()

	if h.onStartCallback != nil {
		h.onStartCallback(server)
	}
}

func (h *testServerHandler) OnOpen(c gain.Conn) {
	if h.onOpenCallback != nil {
		h.onOpenCallback(c)
	}
}

func (h *testServerHandler) OnClose(c gain.Conn) {
	if h.onCloseCallback != nil {
		h.onCloseCallback(c)
	}
}

func (h *testServerHandler) OnRead(conn gain.Conn) {
	if h.onReadCallback != nil {
		h.onReadCallback(conn)
	}
}

func (h *testServerHandler) OnWrite(c gain.Conn) {
	if h.onWriteCallback != nil {
		h.onWriteCallback(c)
	}
}

type afterDialCallback func(*testing.T, net.Conn, int, int)

var deafultAfterDial = func(t *testing.T, conn net.Conn, repeats, clientIndex int) {
	t.Helper()
	err := conn.SetDeadline(time.Now().Add(time.Second * 2))
	Nil(t, err)

	clientIndexBytes := []byte(fmt.Sprintf("cindex%04d", clientIndex))

	for i := 0; i < repeats; i++ {
		var bytesWritten int
		bytesWritten, err = conn.Write(append(clientIndexBytes, []byte("testdata1234567890")...))

		Nil(t, err)
		Equal(t, 28, bytesWritten)
		var buffer [64]byte
		var bytesRead int
		bytesRead, err = conn.Read(buffer[:])

		Nil(t, err)
		Equal(t, 26, bytesRead)
		Equal(t, string(append(clientIndexBytes, "TESTpayload12345"...)),
			string(buffer[:bytesRead]), "CONNFD: %d", getFdFromConn(conn))
	}
}

func dialClient(t *testing.T, protocol string, port int, clientConnChan chan net.Conn) {
	t.Helper()
	conn, err := net.DialTimeout(protocol, fmt.Sprintf("localhost:%d", port), time.Second)
	Nil(t, err)
	NotNil(t, conn)
	clientConnChan <- conn
}

func dialClientRW(t *testing.T, protocol string, port int,
	afterDial afterDialCallback, repeats, clientIndex int, clientConnChan chan net.Conn,
) {
	t.Helper()
	conn, err := net.DialTimeout(protocol, fmt.Sprintf("localhost:%d", port), 2*time.Second)
	Nil(t, err)
	NotNil(t, conn)
	afterDial(t, conn, repeats, clientIndex)
	clientConnChan <- conn
}

func newTestServerHandler(onReadCallback onReadCallback) *testServerHandler {
	testHandler := &testServerHandler{}

	var startedWg sync.WaitGroup

	startedWg.Add(1)
	testHandler.startedWg = &startedWg

	if onReadCallback != nil {
		testHandler.onReadCallback = onReadCallback
	} else {
		testHandler.onReadCallback = defaultTestOnReadCallback
	}

	return testHandler
}

func testServer(t *testing.T, testConfig testServerConfig, architecture gain.ServerArchitecture) {
	t.Helper()

	if testConfig.protocol == "" {
		log.Panic("network protocol is missing")
	}
	opts := []gain.ConfigOption{
		gain.WithLoggerLevel(zerolog.FatalLevel),
		gain.WithAsyncHandler(testConfig.asyncHandler),
		gain.WithGoroutinePool(testConfig.goroutinePool),
		gain.WithCPUAffinity(testConfig.cpuAffinity),
		gain.WithWorkers(testConfig.numberOfWorkers),
		gain.WithCBPF(false),
		gain.WithArchitecture(architecture),
	}

	config := gain.NewConfig(opts...)

	testHandler := newTestServerHandler(testConfig.readHandler)

	server := gain.NewServer(testHandler, config)

	defer func() {
		server.Close()
	}()
	testPort := getTestPort()

	go func() {
		err := server.Start(fmt.Sprintf("%s://localhost:%d", testConfig.protocol, testPort))
		if err != nil {
			log.Panic(err)
		}
	}()

	clientConnChan := make(chan net.Conn, testConfig.numberOfClients)

	testHandler.startedWg.Wait()

	if testConfig.waitForDialAllClients {
		clientConnectWG := new(sync.WaitGroup)
		clientConnectWG.Add(testConfig.numberOfClients)
		testHandler.onOpenCallback = func(c gain.Conn) {
			clientConnectWG.Done()
		}

		for i := 0; i < testConfig.numberOfClients; i++ {
			go dialClient(t, testConfig.protocol, testPort, clientConnChan)
		}
		clientConnectWG.Wait()
		Equal(t, testConfig.numberOfClients, server.ActiveConnections())

		for i := 0; i < testConfig.numberOfClients; i++ {
			conn := <-clientConnChan
			NotNil(t, conn)

			if tcpConn, ok := conn.(*net.TCPConn); ok {
				err := tcpConn.SetLinger(0)
				Nil(t, err)
			}
		}
	} else {
		var clientConnectWG *sync.WaitGroup
		if testConfig.protocol == gainNet.TCP {
			clientConnectWG = new(sync.WaitGroup)
			clientConnectWG.Add(testConfig.numberOfClients)
		}
		clientRWWG := new(sync.WaitGroup)
		if testConfig.writesCount == 0 {
			testConfig.writesCount = 1
		}
		clientRWWG.Add(testConfig.numberOfClients * testConfig.writesCount)
		if testConfig.protocol == gainNet.TCP {
			testHandler.onOpenCallback = func(c gain.Conn) {
				clientConnectWG.Done()
			}
		}
		testHandler.onWriteCallback = func(c gain.Conn) {
			clientRWWG.Done()
		}
		afterDial := deafultAfterDial
		if testConfig.afterDial != nil {
			afterDial = testConfig.afterDial
		}
		for i := 0; i < testConfig.numberOfClients; i++ {
			go func(clientIndex int) {
				dialClientRW(t, testConfig.protocol, testPort, afterDial, testConfig.writesCount, clientIndex, clientConnChan)
			}(i)
		}
		if testConfig.protocol == gainNet.TCP {
			clientConnectWG.Wait()
		}
		clientRWWG.Wait()
		for i := 0; i < testConfig.numberOfClients; i++ {
			conn := <-clientConnChan
			NotNil(t, conn)
			if tcpConn, ok := conn.(*net.TCPConn); ok {
				err := tcpConn.SetLinger(0)
				Nil(t, err)
			}
		}
	}
}

var randomDataSize128 = make([]byte, 128)

type RingBufferTestDataHandler struct {
	t            *testing.T
	testFinished atomic.Bool
}

func (r *RingBufferTestDataHandler) OnRead(conn gain.Conn) {
	buffer := make([]byte, 128)
	bytesRead, readErr := conn.Read(buffer)

	if !r.testFinished.Load() {
		Equal(r.t, 128, bytesRead)

		if readErr != nil {
			log.Panic(readErr)
		}
		bytesWritten, writeErr := conn.Write(randomDataSize128)
		Equal(r.t, 128, bytesWritten)

		if writeErr != nil {
			log.Panic(writeErr)
		}
	}
}

func testRingBuffer(t *testing.T, protocol string, architecture gain.ServerArchitecture) {
	t.Helper()
	handler := RingBufferTestDataHandler{
		t: t,
	}
	bytesRandom, err := rand.Read(randomDataSize128)
	Nil(t, err)
	Equal(t, 128, bytesRandom)
	writesCount := 1000
	testServer(t, testServerConfig{
		numberOfClients: 1,
		numberOfWorkers: 1,
		protocol:        protocol,
		readHandler:     handler.OnRead,
		writesCount:     writesCount,
		afterDial: func(t *testing.T, conn net.Conn, _, _ int) {
			t.Helper()
			deadlineErr := conn.SetDeadline(time.Now().Add(time.Second * 1))
			Nil(t, deadlineErr)
			var buffer [256]byte
			for i := 0; i < writesCount; i++ {
				bytesWritten, writeErr := conn.Write(randomDataSize128)
				Nil(t, writeErr)
				Equal(t, 128, bytesWritten)
				bytesRead, readErr := conn.Read(buffer[:])
				Nil(t, readErr)
				Equal(t, 128, bytesRead)
				Equal(t, randomDataSize128, buffer[:bytesRead])
			}
			handler.testFinished.Store(true)
		},
	}, architecture)
}

func testCloseServer(t *testing.T, network string, architecture gain.ServerArchitecture, doubleClose bool) {
	t.Helper()
	testHandler := newConnServerTester(10, false)
	server, port := newTestConnServer(t, network, false, architecture, testHandler.testServerHandler)
	clientsGroup := newTestConnClientGroup(t, network, port, 10)
	clientsGroup.Dial()

	data := make([]byte, 1024)
	_, err := rand.Read(data)
	Nil(t, err)
	clientsGroup.Write(data)
	buffer := make([]byte, 1024)
	clientsGroup.Read(buffer)

	testHandler.waitForWrites()
	clientsGroup.Close()
	server.Close()

	if doubleClose {
		server.Close()
	}
}

func testCloseServerWithConnectedClients(t *testing.T, architecture gain.ServerArchitecture) {
	t.Helper()
	testHandler := newConnServerTester(10, false)
	server, port := newTestConnServer(t, gainNet.TCP, false, architecture, testHandler.testServerHandler)

	clientsGroup := newTestConnClientGroup(t, gainNet.TCP, port, 10)
	clientsGroup.Dial()

	data := make([]byte, 1024)
	_, err := rand.Read(data)
	Nil(t, err)
	clientsGroup.Write(data)
	buffer := make([]byte, 1024)
	clientsGroup.Read(buffer)

	testHandler.waitForWrites()
	server.Close()
}

func testCloseConn(t *testing.T, async bool, architecture gain.ServerArchitecture) {
	t.Helper()
	testHandler := newTestServerHandler(func(conn gain.Conn) {
		buf, err := conn.Next(-1)
		if err != nil {
			log.Panic(err)
		}

		_, err = conn.Write(buf)
		if err != nil {
			log.Panic(err)
		}

		err = conn.Close()
		if err != nil {
			log.Panic(err)
		}
	})

	server, port := newTestConnServer(t, gainNet.TCP, async, architecture, testHandler)

	var clientDoneWg sync.WaitGroup

	clientDoneWg.Add(1)

	go func(wg *sync.WaitGroup) {
		conn, cErr := net.DialTimeout(gainNet.TCP, fmt.Sprintf("localhost:%d", port), time.Second)
		Nil(t, cErr)
		NotNil(t, conn)
		testData := []byte("testdata1234567890")
		bytesN, cErr := conn.Write(testData)
		Nil(t, cErr)
		Equal(t, len(testData), bytesN)
		buffer := make([]byte, len(testData))
		bytesN, cErr = conn.Read(buffer)
		Nil(t, cErr)
		Equal(t, len(testData), bytesN)
		Equal(t, testData, buffer)
		bytesN, cErr = conn.Write(testData)
		Nil(t, cErr)
		Equal(t, len(testData), bytesN)
		bytesN, cErr = conn.Read(buffer)
		Equal(t, io.EOF, cErr)
		Equal(t, 0, bytesN)
		wg.Done()
	}(&clientDoneWg)

	clientDoneWg.Wait()
	server.Close()
}
