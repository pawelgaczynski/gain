// Copyright (c) 2023 Paweł Gaczyński
// Copyright (c) 2020 Andy Pan
// Copyright (c) 2017 Max Riveiro
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

package socket

import (
	"errors"
	"fmt"
	"net"
	"os"

	gainErrors "github.com/pawelgaczynski/gain/pkg/errors"
	gainNet "github.com/pawelgaczynski/gain/pkg/net"
	"golang.org/x/sys/unix"
)

var listenerBacklogMaxSize = maxListenerBacklog()

// GetTCPSockAddr the structured addresses based on the protocol and raw address.
//
//nolint:dupl // dupl marks this incorrectly as duplicate of GetUDPSockAddr
func GetTCPSockAddr(proto, addr string) (unix.Sockaddr, int, *net.TCPAddr, bool, error) {
	var (
		sockAddr   unix.Sockaddr
		family     int
		tcpAddr    *net.TCPAddr
		ipv6only   bool
		err        error
		tcpVersion string
	)

	tcpAddr, err = net.ResolveTCPAddr(proto, addr)
	if err != nil {
		return sockAddr, family, tcpAddr, ipv6only, fmt.Errorf("resolveTCPAddr error: %w", err)
	}

	tcpVersion, err = determineTCPProto(proto, tcpAddr)
	if err != nil {
		return sockAddr, family, tcpAddr, ipv6only, err
	}

	switch tcpVersion {
	case gainNet.TCP4:
		family = unix.AF_INET
		sockAddr, err = ipToSockaddr(family, tcpAddr.IP, tcpAddr.Port, "")

	case gainNet.TCP6:
		ipv6only = true

		fallthrough

	case gainNet.TCP:
		family = unix.AF_INET6
		sockAddr, err = ipToSockaddr(family, tcpAddr.IP, tcpAddr.Port, tcpAddr.Zone)

	default:
		err = gainErrors.ErrUnsupportedProtocol
	}

	return sockAddr, family, tcpAddr, ipv6only, err
}

func determineTCPProto(proto string, addr *net.TCPAddr) (string, error) {
	// If the protocol is set to "tcp", we try to determine the actual protocol
	// version from the size of the resolved IP address. Otherwise, we simple use
	// the protocol given to us by the caller.
	if addr.IP.To4() != nil {
		return gainNet.TCP4, nil
	}

	if addr.IP.To16() != nil {
		return gainNet.TCP6, nil
	}

	switch proto {
	case gainNet.TCP, gainNet.TCP4, gainNet.TCP6:
		return proto, nil
	}

	return "", gainErrors.ErrUnsupportedTCPProtocol
}

// tcpSocket creates an endpoint for communication and returns a file descriptor that refers to that endpoint.
func tcpSocket(proto, addr string, passive bool, sockOpts ...Option) (int, net.Addr, error) {
	var (
		fd       int
		netAddr  net.Addr
		err      error
		family   int
		ipv6only bool
		sockAddr unix.Sockaddr
	)

	if sockAddr, family, netAddr, ipv6only, err = GetTCPSockAddr(proto, addr); err != nil {
		return fd, netAddr, err
	}

	if fd, err = sysSocket(family, unix.SOCK_STREAM, unix.IPPROTO_TCP); err != nil {
		err = os.NewSyscallError("socket", err)

		return fd, netAddr, err
	}

	defer func() {
		// ignore EINPROGRESS for non-blocking socket connect, should be processed by caller
		if err != nil {
			var syscallErr *os.SyscallError
			if errors.As(err, &syscallErr) && errors.Is(syscallErr.Err, unix.EINPROGRESS) {
				return
			}
			_ = unix.Close(fd)
		}
	}()

	if family == unix.AF_INET6 && ipv6only {
		if err = SetIPv6Only(fd, 1); err != nil {
			return fd, netAddr, err
		}
	}

	for _, sockOpt := range sockOpts {
		if err = sockOpt.SetSockOpt(fd, sockOpt.Opt); err != nil {
			return fd, netAddr, err
		}
	}

	if passive {
		if err = os.NewSyscallError("bind", unix.Bind(fd, sockAddr)); err != nil {
			return fd, netAddr, err
		}
		// Set backlog size to the maximum.
		err = os.NewSyscallError("listen", unix.Listen(fd, listenerBacklogMaxSize))
	} else {
		err = os.NewSyscallError("connect", unix.Connect(fd, sockAddr))
	}

	return fd, netAddr, err
}
