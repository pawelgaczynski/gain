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

package gain

import (
	"net"
	"runtime"
	"strings"

	"github.com/pawelgaczynski/gain/pkg/errors"
	gainNet "github.com/pawelgaczynski/gain/pkg/net"
	"github.com/pawelgaczynski/gain/pkg/socket"
)

func parseProtoAddr(addr string) (string, string) {
	network := gainNet.TCP

	address := strings.ToLower(addr)
	if strings.Contains(address, "://") {
		pair := strings.Split(address, "://")
		network = pair[0]
		address = pair[1]
	}

	return network, address
}

type listener struct {
	fd               int
	addr             net.Addr
	address, network string
	sockOpts         []socket.Option
}

func (ln *listener) normalize() error {
	var err error

	switch ln.network {
	case gainNet.TCP, gainNet.TCP4, gainNet.TCP6:
		ln.fd, ln.addr, err = socket.TCPSocket(ln.network, ln.address, true, ln.sockOpts...)
		ln.network = gainNet.TCP

	case gainNet.UDP, gainNet.UDP4, gainNet.UDP6:
		ln.fd, ln.addr, err = socket.UDPSocket(ln.network, ln.address, false, ln.sockOpts...)
		ln.network = gainNet.UDP

	default:
		err = errors.ErrUnsupportedProtocol
	}

	return err
}

func initListener(network, addr string, config Config) (*listener, error) {
	var (
		ln       *listener
		err      error
		sockOpts []socket.Option
	)

	sockOpt := socket.Option{SetSockOpt: socket.SetReuseport, Opt: 1}
	sockOpts = append(sockOpts, sockOpt)
	sockOpt = socket.Option{SetSockOpt: socket.SetReuseAddr, Opt: 1}
	sockOpts = append(sockOpts, sockOpt)

	if strings.HasPrefix(network, gainNet.TCP) {
		sockOpt = socket.Option{SetSockOpt: socket.SetNoDelay, Opt: 1}
		sockOpts = append(sockOpts, sockOpt)

		sockOpt = socket.Option{SetSockOpt: socket.SetQuickAck, Opt: 1}
		sockOpts = append(sockOpts, sockOpt)

		sockOpt = socket.Option{SetSockOpt: socket.SetFastOpen, Opt: 1}
		sockOpts = append(sockOpts, sockOpt)
	}

	if config.SocketRecvBufferSize > 0 {
		sockOpt = socket.Option{SetSockOpt: socket.SetRecvBuffer, Opt: config.SocketRecvBufferSize}
		sockOpts = append(sockOpts, sockOpt)
	}

	if config.SocketSendBufferSize > 0 {
		sockOpt = socket.Option{SetSockOpt: socket.SetSendBuffer, Opt: config.SocketSendBufferSize}
		sockOpts = append(sockOpts, sockOpt)
	}

	ln = &listener{network: network, address: addr, sockOpts: sockOpts}
	err = ln.normalize()

	if config.CBPFilter {
		err = newFilter(uint32(runtime.NumCPU())).applyTo(ln.fd)
		if err != nil {
			return ln, err
		}
	}

	return ln, err
}
