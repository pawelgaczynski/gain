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

import (
	"github.com/pkg/errors"
)

type EventHandler interface {
	// OnStart fires when the server is ready for accepting new connections.
	OnStart(server Server)
	// OnAccept fires when a new TCP connection has been opened (will not be fired for UDP protocol).
	// This is good place to set the socket options, such as TCP keepalive.
	// The incoming data buffer of the connection will be empty,
	// so the attempt to read the data will never succeed, but you can perform a write at this point
	// The Conn c has information about the connection such as it's local and remote address.
	OnAccept(c Conn)
	// OnRead fires when a socket receives n bytes of data from the peer.
	//
	// Call c.Read() of Conn c to read incoming data from the peer. call c.Write() to send data to the remote peer.
	OnRead(c Conn, n int)
	// OnWrite fires right after a n bytes is written to the peer socket.
	OnWrite(c Conn, n int)
	// OnClose fires when a TCP connection has been closed (will not be fired for UDP protocol).
	// The parameter err is the last known connection error.
	OnClose(c Conn, err error)
}

// DefaultEventHandler is a default implementation for all of the EventHandler callbacks (do nothing).
// Compose it with your own implementation of EventHandler and you won't need to implement all callbacks.
type DefaultEventHandler struct{}

func (e DefaultEventHandler) OnStart(server Server)     {}
func (e DefaultEventHandler) OnAccept(c Conn)           {}
func (e DefaultEventHandler) OnClose(c Conn, err error) {}
func (e DefaultEventHandler) OnRead(c Conn, n int)      {}
func (e DefaultEventHandler) OnWrite(c Conn, n int)     {}

// ListenAndServe starts a server with a given address and event handler.
// The server can be configured with additional options.
func ListenAndServe(address string, eventHandler EventHandler, options ...ConfigOption) error {
	server := NewServer(eventHandler, NewConfig(options...))

	return errors.Wrapf(server.Start(address), "starting server error")
}
