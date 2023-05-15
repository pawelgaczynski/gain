// Copyright (c) 2023 Paweł Gaczyński
// Copyright (c) 2019 Andy Pan
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

package errors

import (
	"errors"
	"fmt"
)

var (
	// ErrUnsupportedProtocol occurs when trying to use protocol that is not supported.
	ErrUnsupportedProtocol = errors.New("only unix, tcp/tcp4/tcp6, udp/udp4/udp6 are supported")
	// ErrUnsupportedTCPProtocol occurs when trying to use an unsupported TCP protocol.
	ErrUnsupportedTCPProtocol = errors.New("only tcp/tcp4/tcp6 are supported")
	// ErrUnsupportedUDPProtocol occurs when trying to use an unsupported UDP protocol.
	ErrUnsupportedUDPProtocol = errors.New("only udp/udp4/udp6 are supported")
	// ErrNoIPv4AddressOnInterface occurs when an IPv4 multicast address is set on an interface but IPv4 is not configured.
	ErrNoIPv4AddressOnInterface = errors.New("no IPv4 address on interface")
	// ErrNotSupported occurs when not supported feature is used.
	ErrNotSupported = errors.New("not supported")
	// ErrSkippable indicates an error that can be skipped and not handled as an usual flow breaking error.
	ErrSkippable = errors.New("skippable")
	// ErrIsEmpty indicates that data holder, data magazine or buffer is empty.
	ErrIsEmpty = errors.New("is empty")
	// ErrConnectionAlreadyClosed when trying to close already closed connection.
	ErrConnectionAlreadyClosed = errors.New("connection already closed")
	// ErrConnectionAlreadyClosed when trying to work with closed connection.
	ErrConnectionClosed = errors.New("connection closed")
	// ErrConnectionIsMissing occurs when trying to access connection that is missing.
	// For TCP connections key is the file descriptor.
	ErrConnectionIsMissing = errors.New("connection is missing")
	// ErrOpNotAvailableInMode occurs when trying to run operation that is not available in current mode.
	ErrOpNotAvailableInMode = errors.New("op is not available in mode")
	// ErrConnectionQueueIsNil occurs when trying to access connection queue that is not initialized.
	ErrConnectionQueueIsNil = errors.New("connection queue is nil")
	// ErrUnknownConnectionState occurs when connection state is unknown.
	ErrUnknownConnectionState = errors.New("unknown connection state")
	// ErrInvalidTimeDuration occure when specyfied time duration is not valid.
	ErrInvalidTimeDuration = errors.New("invalid time duration")
	// ErrInvalidState occurs when operation is called in invalid state.
	ErrInvalidState = errors.New("invalid state")
	// ErrAddressNotFound occurs when network address of fd could not be found.
	ErrAddressNotFound = errors.New("address could not be found")
	// ErrServerAlreadyRunning occurs when trying to start already running server.
	ErrServerAlreadyRunning = errors.New("server already running")
)

func ErrorConnectionIsMissing(key int) error {
	return fmt.Errorf("%w, key: %d", ErrConnectionIsMissing, key)
}

func ErrorOpNotAvailableInMode(op, mode string) error {
	return fmt.Errorf("%w, op: %s, mode: %s", ErrOpNotAvailableInMode, op, mode)
}

func ErrorUnknownConnectionState(state int) error {
	return fmt.Errorf("%w, state: %d", ErrUnknownConnectionState, state)
}

func ErrorAddressNotFound(fd int) error {
	return fmt.Errorf("%w, fd: %d", ErrAddressNotFound, fd)
}
