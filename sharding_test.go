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
	"testing"
	"time"

	"github.com/pawelgaczynski/gain"
	gainNet "github.com/pawelgaczynski/gain/pkg/net"
	. "github.com/stretchr/testify/require"
)

func TestShardingSingleWorkerSingleClient(t *testing.T) {
	testServer(t, testServerConfig{
		protocol:        gainNet.TCP,
		numberOfClients: 1,
		numberOfWorkers: 1,
	}, gain.SocketSharding)
}

func TestShardingSingleWorkerManyClients(t *testing.T) {
	testServer(t, testServerConfig{
		protocol:        gainNet.TCP,
		numberOfClients: 8,
		numberOfWorkers: 1,
	}, gain.SocketSharding)
}

func TestShardingManyWorkersManyClients(t *testing.T) {
	testServer(t, testServerConfig{
		protocol:        gainNet.TCP,
		numberOfClients: 16,
		numberOfWorkers: 8,
	}, gain.SocketSharding)
}

func TestShardingAsyncHandler(t *testing.T) {
	testServer(t, testServerConfig{
		protocol:        gainNet.TCP,
		numberOfClients: 8,
		numberOfWorkers: 8,
		asyncHandler:    true,
	}, gain.SocketSharding)
}

func TestShardingAsyncHandlerWithPool(t *testing.T) {
	testServer(t, testServerConfig{
		protocol:        gainNet.TCP,
		numberOfClients: 8,
		numberOfWorkers: 8,
		asyncHandler:    true,
		goroutinePool:   true,
	}, gain.SocketSharding)
}

func TestShardingWaitForDialAllClients(t *testing.T) {
	testServer(t, testServerConfig{
		protocol:              gainNet.TCP,
		numberOfClients:       8,
		numberOfWorkers:       8,
		waitForDialAllClients: true,
	}, gain.SocketSharding)
}

func TestShardingUDPSingleWorkerSingleClient(t *testing.T) {
	testServer(t, testServerConfig{
		protocol:        gainNet.UDP,
		numberOfClients: 1,
		numberOfWorkers: 1,
		writesCount:     2,
	}, gain.SocketSharding)
}

func TestShardingUDPSingleWorkerManyClients(t *testing.T) {
	testServer(t, testServerConfig{
		protocol:        gainNet.UDP,
		numberOfClients: 8,
		numberOfWorkers: 1,
	}, gain.SocketSharding)
}

func TestShardingUDPManyWorkersManyClients(t *testing.T) {
	testServer(t, testServerConfig{
		protocol:        gainNet.UDP,
		numberOfClients: 16,
		numberOfWorkers: 8,
	}, gain.SocketSharding)
}

func TestShardingUDPAsyncHandler(t *testing.T) {
	testServer(t, testServerConfig{
		protocol:        gainNet.UDP,
		numberOfClients: 8,
		numberOfWorkers: 8,
		asyncHandler:    true,
	}, gain.SocketSharding)
}

func TestShardingUDPAsyncHandlerWithPool(t *testing.T) {
	testServer(t, testServerConfig{
		protocol:        gainNet.UDP,
		numberOfClients: 8,
		numberOfWorkers: 8,
		asyncHandler:    true,
		goroutinePool:   true,
	}, gain.SocketSharding)
}

func TestShardingTCPRingBuffer(t *testing.T) {
	testRingBuffer(t, gainNet.TCP, gain.SocketSharding)
}

func TestShardingUDPRingBuffer(t *testing.T) {
	testRingBuffer(t, gainNet.UDP, gain.SocketSharding)
}

func TestShardingConnectionHandling(t *testing.T) {
	testConnectionHandling(t, gain.SocketSharding)
}

func TestShardingTCPCloseSever(t *testing.T) {
	testCloseServer(t, gainNet.TCP, gain.SocketSharding, false)
}

func TestShardingUDPCloseSever(t *testing.T) {
	testCloseServer(t, gainNet.UDP, gain.SocketSharding, false)
}

func TestShardingTCPDoubleCloseSever(t *testing.T) {
	testCloseServer(t, gainNet.TCP, gain.SocketSharding, true)
}

func TestShardingUDPDoubleCloseSever(t *testing.T) {
	testCloseServer(t, gainNet.UDP, gain.SocketSharding, true)
}

func TestShardingCloseSeverWithConnectedClients(t *testing.T) {
	testCloseServerWithConnectedClients(t, gain.SocketSharding)
}

func TestShardingReleasingUDPConns(t *testing.T) {
	testHandler := newConnServerTester(gainNet.UDP, 10, true)
	server, port := newTestConnServer(t, gainNet.UDP, false, gain.SocketSharding, testHandler.testServerHandler)

	clientsGroup := newTestConnClientGroup(t, gainNet.UDP, port, 10)
	clientsGroup.Dial()

	go func() {
		for {
			data := make([]byte, 1024)
			_, err := rand.Read(data)
			Nil(t, err)
			clientsGroup.Write(data)
			buffer := make([]byte, 1024)
			clientsGroup.Read(buffer)
		}
	}()

	Equal(t, 0, server.ActiveConnections())
	wg := testHandler.writeWG
	wg.Wait()
	server.Shutdown()
}

func TestShardingTCPCloseConnSync(t *testing.T) {
	testCloseConn(t, false, gain.SocketSharding, false)
}

func TestShardingTCPCloseConnAsync(t *testing.T) {
	testCloseConn(t, true, gain.SocketSharding, false)
}

func TestShardingTCPOnlyCloseConnSync(t *testing.T) {
	testCloseConn(t, false, gain.SocketSharding, true)
}

func TestShardingTCPOnlyCloseConnAsync(t *testing.T) {
	testCloseConn(t, true, gain.SocketSharding, true)
}

func TestShardingTCPConnAddress(t *testing.T) {
	testConnAddress(t, gainNet.TCP, gain.SocketSharding)
}

func TestShardingUDPConnAddress(t *testing.T) {
	testConnAddress(t, gainNet.UDP, gain.SocketSharding)
}

func TestShardingTCPLargeRead(t *testing.T) {
	testLargeRead(t, gainNet.TCP, gain.SocketSharding)
}

func TestShardingTCPMultipleReads(t *testing.T) {
	testMultipleReads(t, gainNet.TCP, false, gain.SocketSharding)
}

func TestShardingTCPAsyncHandlerMultipleReads(t *testing.T) {
	testMultipleReads(t, gainNet.TCP, true, gain.SocketSharding)
}

func TestShardingManyWorkersManyClientsWithCBPF(t *testing.T) {
	testServer(t, testServerConfig{
		protocol:        gainNet.TCP,
		numberOfClients: 16,
		numberOfWorkers: 8,
		configOptions: []gain.ConfigOption{
			gain.WithCBPF(true),
		},
	}, gain.SocketSharding)
}

func TestShardingManyWorkersManyClientsSocketBuffers(t *testing.T) {
	testServer(t, testServerConfig{
		protocol:        gainNet.TCP,
		numberOfClients: 16,
		numberOfWorkers: 8,
		configOptions: []gain.ConfigOption{
			gain.WithSocketRecvBufferSize(2048),
			gain.WithSocketSendBufferSize(2048),
		},
	}, gain.SocketSharding)
}

func TestShardingManyWorkersManyClientsTCPKeepAlive(t *testing.T) {
	testServer(t, testServerConfig{
		protocol:        gainNet.TCP,
		numberOfClients: 16,
		numberOfWorkers: 8,
		configOptions: []gain.ConfigOption{
			gain.WithTCPKeepAlive(time.Minute),
		},
	}, gain.SocketSharding)
}

func TestShardingManyWorkersManyClientsCPUAffinity(t *testing.T) {
	testServer(t, testServerConfig{
		protocol:        gainNet.TCP,
		numberOfClients: 16,
		numberOfWorkers: 8,
		configOptions: []gain.ConfigOption{
			gain.WithCPUAffinity(true),
		},
	}, gain.SocketSharding)
}
