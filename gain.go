// Copyright (c) 2023 Paweł Gaczyński
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
//nolint:revive
package gain

type EventHandler interface {
	// OnStart fires when the server is ready for accepting new connections.
	OnStart(server Server)
	// OnOpen fires when a new connection has been opened.
	// The Conn c has information about the connection such as it's local and remote address.
	OnOpen(c Conn)
	// OnRead fires when a socket receives data from the peer.
	// Call c.Read() of Conn c to read incoming data from the peer.
	OnRead(c Conn)
	// OnWrite fires right after a packet is written to the peer socket.
	OnWrite(c Conn)
	// OnClose fires when a connection has been closed.
	OnClose(c Conn)
}

// DefaultEventHandler is a default implementation for all of the EventHandler callbacks (do nothing).
// Compose it with your own implementation of EventHandler and you won't need to implement all callbacks.
type DefaultEventHandler struct{}

func (e DefaultEventHandler) OnStart(server Server) {}
func (e DefaultEventHandler) OnOpen(c Conn)         {}
func (e DefaultEventHandler) OnClose(c Conn)        {}
func (e DefaultEventHandler) OnRead(c Conn)         {}
func (e DefaultEventHandler) OnWrite(c Conn)        {}
