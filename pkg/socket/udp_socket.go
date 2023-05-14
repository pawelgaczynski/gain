// Copyright (c) 2023 Paweł Gaczyński
// Copyright (c) 2020 Andy Pan
// Copyright (c) 2017 Max Riveiro
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

// GetUDPSockAddr the structured addresses based on the protocol and raw address.
//
//nolint:dupl // dupl marks this incorrectly as duplicate of GetTCPSockAddr
func GetUDPSockAddr(proto, addr string) (unix.Sockaddr, int, *net.UDPAddr, bool, error) {
	var (
		sockAddr   unix.Sockaddr
		family     int
		udpAddr    *net.UDPAddr
		ipv6only   bool
		err        error
		udpVersion string
	)

	udpAddr, err = net.ResolveUDPAddr(proto, addr)
	if err != nil {
		return sockAddr, family, udpAddr, ipv6only, fmt.Errorf("resolveUDPAddr error: %w", err)
	}

	udpVersion, err = determineUDPProto(proto, udpAddr)
	if err != nil {
		return sockAddr, family, udpAddr, ipv6only, err
	}

	switch udpVersion {
	case gainNet.UDP4:
		family = unix.AF_INET
		sockAddr, err = ipToSockaddr(family, udpAddr.IP, udpAddr.Port, "")

	case gainNet.UDP6:
		ipv6only = true

		fallthrough

	case gainNet.UDP:
		family = unix.AF_INET6
		sockAddr, err = ipToSockaddr(family, udpAddr.IP, udpAddr.Port, udpAddr.Zone)

	default:
		err = gainErrors.ErrUnsupportedProtocol
	}

	return sockAddr, family, udpAddr, ipv6only, err
}

func determineUDPProto(proto string, addr *net.UDPAddr) (string, error) {
	// If the protocol is set to "udp", we try to determine the actual protocol
	// version from the size of the resolved IP address. Otherwise, we simple use
	// the protocol given to us by the caller.
	if addr.IP.To4() != nil {
		return gainNet.UDP4, nil
	}

	if addr.IP.To16() != nil {
		return gainNet.UDP6, nil
	}

	switch proto {
	case gainNet.UDP, gainNet.UDP4, gainNet.UDP6:
		return proto, nil
	}

	return "", gainErrors.ErrUnsupportedUDPProtocol
}

// udpSocket creates an endpoint for communication and returns a file descriptor that refers to that endpoint.
func udpSocket(proto, addr string, connect bool, sockOpts ...Option) (int, net.Addr, error) {
	var (
		fd       int
		netAddr  net.Addr
		err      error
		family   int
		ipv6only bool
		sockAddr unix.Sockaddr
	)

	if sockAddr, family, netAddr, ipv6only, err = GetUDPSockAddr(proto, addr); err != nil {
		return fd, netAddr, err
	}

	if fd, err = sysSocket(family, unix.SOCK_DGRAM, unix.IPPROTO_UDP); err != nil {
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

	// Allow broadcast.
	if err = os.NewSyscallError("setsockopt", unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_BROADCAST, 1)); err != nil {
		return fd, netAddr, err
	}

	for _, sockOpt := range sockOpts {
		if err = sockOpt.SetSockOpt(fd, sockOpt.Opt); err != nil {
			return fd, netAddr, err
		}
	}

	if connect {
		err = os.NewSyscallError("connect", unix.Connect(fd, sockAddr))
	} else {
		err = os.NewSyscallError("bind", unix.Bind(fd, sockAddr))
	}

	return fd, netAddr, err
}
