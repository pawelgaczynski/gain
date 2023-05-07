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
	"fmt"
	"log"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pawelgaczynski/gain"
	"github.com/rs/zerolog"
	. "github.com/stretchr/testify/require"
)

type connServerTester struct {
	*testServerHandler
	mutex                  sync.Mutex
	writeWG                *sync.WaitGroup
	writeCount             uint32
	targetWriteCount       uint32
	removeWGAfterMinWrites bool
}

func (t *connServerTester) waitForWrites() {
	t.writeWG.Wait()
}

func (t *connServerTester) onReadCallback(conn gain.Conn, n int) {
	buf, _ := conn.Next(n)
	_, _ = conn.Write(buf)
}

func (t *connServerTester) onWriteCallback(_ gain.Conn, _ int) {
	if t.writeWG != nil {
		t.mutex.Lock()

		t.writeCount++
		if t.writeCount >= t.targetWriteCount {
			t.writeWG.Done()

			if t.removeWGAfterMinWrites {
				t.writeWG = nil
			}
		}
		t.mutex.Unlock()
	}
}

func newConnServerTester(writeCount int, removeWGAfterMinWrites bool) *connServerTester {
	connServerTester := &connServerTester{}

	if writeCount > 0 {
		var writeWG sync.WaitGroup

		writeWG.Add(1)
		connServerTester.writeWG = &writeWG
		connServerTester.targetWriteCount = uint32(writeCount)
		connServerTester.removeWGAfterMinWrites = removeWGAfterMinWrites
	}

	testConnHandler := newTestServerHandler(connServerTester.onReadCallback)

	testConnHandler.onWriteCallback = connServerTester.onWriteCallback
	connServerTester.testServerHandler = testConnHandler

	return connServerTester
}

func newEventHandlerTester(callbacks callbacksHolder) *testServerHandler {
	testHandler := &testServerHandler{}

	var (
		startedWg  sync.WaitGroup
		onAcceptWg sync.WaitGroup
		onReadWg   sync.WaitGroup
		onWriteWg  sync.WaitGroup
		onCloseWg  sync.WaitGroup
	)

	startedWg.Add(1)
	testHandler.startedWg = &startedWg
	testHandler.onAcceptWg = &onAcceptWg
	testHandler.onReadWg = &onReadWg
	testHandler.onWriteWg = &onWriteWg
	testHandler.onCloseWg = &onCloseWg

	testHandler.onStartCallback = callbacks.onStartCallback
	testHandler.onAcceptCallback = callbacks.onAcceptCallback
	testHandler.onReadCallback = callbacks.onReadCallback
	testHandler.onWriteCallback = callbacks.onWriteCallback
	testHandler.onCloseCallback = callbacks.onCloseCallback

	return testHandler
}

type testConnClient struct {
	t       *testing.T
	conn    net.Conn
	network string
	port    int
	idx     int
}

func (c *testConnClient) Dial() {
	conn, err := net.DialTimeout(c.network, fmt.Sprintf("localhost:%d", c.port), time.Millisecond*500)
	Nil(c.t, err)
	NotNil(c.t, conn)
	c.conn = conn
}

func (c *testConnClient) Close() {
	err := c.conn.Close()
	Nil(c.t, err)
}

func (c *testConnClient) SetDeadline(t time.Time) {
	err := c.conn.SetDeadline(t)
	Nil(c.t, err)
}

func (c *testConnClient) Write(buffer []byte) {
	bytesWritten, writeErr := c.conn.Write(buffer)
	Nil(c.t, writeErr)
	Equal(c.t, len(buffer), bytesWritten)
}

func (c *testConnClient) Read(buffer []byte) {
	bytesRead, readErr := c.conn.Read(buffer)
	Nil(c.t, readErr)
	Equal(c.t, len(buffer), bytesRead)
}

func newTestConnClient(t *testing.T, idx int, network string, port int) *testConnClient {
	t.Helper()

	return &testConnClient{
		t:       t,
		network: network,
		port:    port,
		idx:     idx,
	}
}

type testConnClientGroup struct {
	clients []*testConnClient
}

func (c *testConnClientGroup) Dial() {
	for i := 0; i < len(c.clients); i++ {
		c.clients[i].Dial()
	}
}

func (c *testConnClientGroup) Close() {
	for i := 0; i < len(c.clients); i++ {
		c.clients[i].Close()
	}
}

func (c *testConnClientGroup) SetDeadline(t time.Time) {
	for i := 0; i < len(c.clients); i++ {
		c.clients[i].SetDeadline(t)
	}
}

func (c *testConnClientGroup) Write(buffer []byte) {
	for i := 0; i < len(c.clients); i++ {
		c.clients[i].Write(buffer)
	}
}

func (c *testConnClientGroup) Read(buffer []byte) {
	for i := 0; i < len(c.clients); i++ {
		c.clients[i].Read(buffer)
	}
}

func newTestConnClientGroup(t *testing.T, network string, port int, n int) *testConnClientGroup {
	t.Helper()
	group := &testConnClientGroup{
		clients: make([]*testConnClient, n),
	}

	for i := 0; i < n; i++ {
		group.clients[i] = newTestConnClient(t, i, network, port)
	}

	return group
}

func newTestConnServer(
	t *testing.T, network string, async bool, architecture gain.ServerArchitecture, eventHandler *testServerHandler,
) (gain.Server, int) {
	t.Helper()
	opts := []gain.ConfigOption{
		gain.WithLoggerLevel(zerolog.FatalLevel),
		gain.WithWorkers(4),
		gain.WithArchitecture(architecture),
		gain.WithAsyncHandler(async),
	}

	config := gain.NewConfig(opts...)

	server := gain.NewServer(eventHandler, config)
	testPort := getTestPort()

	go func() {
		err := server.Start(fmt.Sprintf("%s://localhost:%d", network, testPort))
		if err != nil {
			log.Panic(err)
		}
	}()

	eventHandler.startedWg.Wait()

	return server, int(port)
}

func getIPAndPort(addr net.Addr) (string, int) {
	switch addr := addr.(type) {
	case *net.UDPAddr:
		return addr.IP.String(), addr.Port
	case *net.TCPAddr:
		return addr.IP.String(), addr.Port
	}

	return "", 0
}

func testConnAddress(
	t *testing.T, network string, architecture gain.ServerArchitecture,
) {
	t.Helper()
	numberOfClients := 10
	opts := []gain.ConfigOption{
		gain.WithLoggerLevel(zerolog.FatalLevel),
		gain.WithWorkers(4),
		gain.WithArchitecture(architecture),
	}

	config := gain.NewConfig(opts...)

	out, err := exec.Command("bash", "-c", "sysctl net.ipv4.ip_local_port_range | awk '{ print $3; }'").Output()
	if err != nil {
		log.Panic(err)
	}

	lowestEphemeralPort, err := strconv.Atoi(strings.ReplaceAll(string(out), "\n", ""))
	if err != nil {
		log.Panic(err)
	}

	verifyAddresses := func(t *testing.T, conn gain.Conn) {
		t.Helper()
		localAddr := conn.LocalAddr()
		NotNil(t, localAddr)

		ip, port := getIPAndPort(localAddr)
		Equal(t, "127.0.0.1", ip)
		Less(t, port, 10000)
		GreaterOrEqual(t, port, 9000)
		remoteAddr := conn.RemoteAddr()

		ip, port = getIPAndPort(remoteAddr)
		NotNil(t, remoteAddr)
		Equal(t, "127.0.0.1", ip)
		GreaterOrEqual(t, port, lowestEphemeralPort)
	}

	var wg sync.WaitGroup

	wg.Add(numberOfClients)

	onReadCallback := func(conn gain.Conn, n int) {
		buf, _ := conn.Next(n)
		_, _ = conn.Write(buf)

		verifyAddresses(t, conn)

		wg.Done()
	}

	testHandler := newTestServerHandler(onReadCallback)

	server := gain.NewServer(testHandler, config)
	testPort := getTestPort()

	go func() {
		serverErr := server.Start(fmt.Sprintf("%s://localhost:%d", network, testPort))
		if err != nil {
			log.Panic(serverErr)
		}
	}()

	testHandler.startedWg.Wait()

	clientsGroup := newTestConnClientGroup(t, network, testPort, numberOfClients)
	clientsGroup.Dial()

	data := make([]byte, 1024)
	_, err = rand.Read(data)
	Nil(t, err)
	clientsGroup.Write(data)
	buffer := make([]byte, 1024)
	clientsGroup.Read(buffer)

	wg.Wait()
	server.Shutdown()
}
