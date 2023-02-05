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

package gain

import (
	"syscall"

	"github.com/pawelgaczynski/gain/iouring"
	"github.com/rs/zerolog"
)

type connCloser struct {
	ring   *iouring.Ring
	logger zerolog.Logger
}

func (c *connCloser) addCloseRequest(fd int) (*iouring.SubmissionQueueEntry, error) {
	entry, err := c.ring.GetSQE()
	if err != nil {
		return nil, err
	}
	entry.PrepareClose(fd)
	entry.UserData = closeConnFlag | uint64(fd)
	return entry, nil
}

func (c *connCloser) addCloseConnRequest(conn *connection) (*iouring.SubmissionQueueEntry, error) {
	entry, err := c.addCloseRequest(conn.fd)
	if err != nil {
		return nil, err
	}
	conn.state = connClose
	return entry, nil
}

func (c *connCloser) syscallShutdownSocket(fileDescriptor int) error {
	return syscall.Shutdown(fileDescriptor, syscall.SHUT_RDWR)
}

func newConnCloser(ring *iouring.Ring, logger zerolog.Logger) *connCloser {
	return &connCloser{
		ring:   ring,
		logger: logger,
	}
}
