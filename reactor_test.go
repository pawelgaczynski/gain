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
	"testing"

	"github.com/pawelgaczynski/gain"
	gainNet "github.com/pawelgaczynski/gain/pkg/net"
)

func TestReactorTCPSingleWorkerSingleClient(t *testing.T) {
	testServer(t, testServerConfig{
		protocol:        gainNet.TCP,
		numberOfClients: 1,
		numberOfWorkers: 1,
	}, gain.Reactor)
}

func TestReactorTCPSingleWorkerManyClients(t *testing.T) {
	testServer(t, testServerConfig{
		protocol:        gainNet.TCP,
		numberOfClients: 8,
		numberOfWorkers: 1,
	}, gain.Reactor)
}

func TestReactorManyWorkersManyClients(t *testing.T) {
	testServer(t, testServerConfig{
		protocol:        gainNet.TCP,
		numberOfClients: 16,
		numberOfWorkers: 8,
	}, gain.Reactor)
}

func TestReactorAsyncHandler(t *testing.T) {
	testServer(t, testServerConfig{
		protocol:        gainNet.TCP,
		numberOfClients: 8,
		numberOfWorkers: 2,
		asyncHandler:    true,
	}, gain.Reactor)
}

func TestReactorAsyncHandlerWithPool(t *testing.T) {
	testServer(t, testServerConfig{
		protocol:        gainNet.TCP,
		numberOfClients: 8,
		numberOfWorkers: 2,
		asyncHandler:    true,
		goroutinePool:   true,
	}, gain.Reactor)
}

func TestReactorWaitForDialAllClients(t *testing.T) {
	testServer(t, testServerConfig{
		protocol:              gainNet.TCP,
		numberOfClients:       8,
		numberOfWorkers:       8,
		waitForDialAllClients: true,
	}, gain.Reactor)
}

func TestReactorTCPRingBuffer(t *testing.T) {
	testRingBuffer(t, gainNet.TCP, gain.Reactor)
}

func TestReactorConnectionHandling(t *testing.T) {
	testConnectionHandling(t, gain.Reactor)
}

func TestReactorTCPCloseSever(t *testing.T) {
	testCloseServer(t, gainNet.TCP, gain.Reactor, false)
}

func TestReactorUDPCloseSever(t *testing.T) {
	testCloseServer(t, gainNet.UDP, gain.Reactor, false)
}

func TestReactorTCPDoubleCloseSever(t *testing.T) {
	testCloseServer(t, gainNet.UDP, gain.Reactor, true)
}

func TestReactorUDPDoubleCloseSever(t *testing.T) {
	testCloseServer(t, gainNet.UDP, gain.Reactor, true)
}

func TestReactorCloseSeverWithConnectedClients(t *testing.T) {
	testCloseServerWithConnectedClients(t, gain.Reactor)
}

func TestReactorTCPCloseConnSync(t *testing.T) {
	testCloseConn(t, false, gain.Reactor)
}

func TestReactorTCPCloseConnAsync(t *testing.T) {
	testCloseConn(t, true, gain.Reactor)
}

func TestReactorTCPConnAddress(t *testing.T) {
	testConnAddress(t, gainNet.TCP, gain.Reactor)
}

func TestReactorTCPLargeRead(t *testing.T) {
	testLargeRead(t, gainNet.TCP, gain.Reactor)
}

func TestReactorTCPMultipleReads(t *testing.T) {
	testMultipleReads(t, gainNet.TCP, false, gain.Reactor)
}

func TestReactorTCPAsyncHandlerMultipleReads(t *testing.T) {
	testMultipleReads(t, gainNet.TCP, true, gain.Reactor)
}
