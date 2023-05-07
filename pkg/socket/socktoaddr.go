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

package socket

import (
	"net"
	"syscall"
	"unsafe"

	bsPool "github.com/pawelgaczynski/gain/pkg/pool/byteslice"
)

var ipv4InIPv6Prefix = []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0xff}

const (
	netIPBytesSize        = 16
	sockAddrPortBitOffset = 8
	decimalBase           = 10
	int32size             = 32
)

func RawAnyToSockaddrInet4(rsa *syscall.RawSockaddrAny) (*syscall.SockaddrInet4, error) {
	if rsa == nil {
		return nil, syscall.EINVAL
	}

	if rsa.Addr.Family != syscall.AF_INET {
		return nil, syscall.EAFNOSUPPORT
	}

	rsaPointer := (*syscall.RawSockaddrInet4)(unsafe.Pointer(rsa))
	sockAddr := new(syscall.SockaddrInet4)
	p := (*[2]byte)(unsafe.Pointer(&rsaPointer.Port))

	sockAddr.Port = int(p[0])<<sockAddrPortBitOffset + int(p[1])
	for i := 0; i < len(sockAddr.Addr); i++ {
		sockAddr.Addr[i] = rsaPointer.Addr[i]
	}

	return sockAddr, nil
}

// SockaddrToTCPOrUnixAddr converts a Sockaddr to a net.TCPAddr or net.UnixAddr.
// Returns nil if conversion fails.
func SockaddrToTCPOrUnixAddr(sysSockAddr syscall.Sockaddr) net.Addr {
	switch sockAddr := sysSockAddr.(type) {
	case *syscall.SockaddrInet4:
		ip := sockaddrInet4ToIP(sockAddr)

		return &net.TCPAddr{IP: ip, Port: sockAddr.Port}

	case *syscall.SockaddrInet6:
		ip, zone := sockaddrInet6ToIPAndZone(sockAddr)

		return &net.TCPAddr{IP: ip, Port: sockAddr.Port, Zone: zone}

	case *syscall.SockaddrUnix:
		return &net.UnixAddr{Name: sockAddr.Name, Net: "unix"}
	}

	return nil
}

// SockaddrToUDPAddr converts a Sockaddr to a net.UDPAddr
// Returns nil if conversion fails.
func SockaddrToUDPAddr(sysSockAddr syscall.Sockaddr) net.Addr {
	switch sockAddr := sysSockAddr.(type) {
	case *syscall.SockaddrInet4:
		ip := sockaddrInet4ToIP(sockAddr)

		return &net.UDPAddr{IP: ip, Port: sockAddr.Port}

	case *syscall.SockaddrInet6:
		ip, zone := sockaddrInet6ToIPAndZone(sockAddr)

		return &net.UDPAddr{IP: ip, Port: sockAddr.Port, Zone: zone}
	}

	return nil
}

// sockaddrInet4ToIPAndZone converts a SockaddrInet4 to a net.IP.
// It returns nil if conversion fails.
func sockaddrInet4ToIP(sa *syscall.SockaddrInet4) net.IP {
	ip := bsPool.Get(netIPBytesSize)
	// ipv4InIPv6Prefix
	copy(ip[0:12], ipv4InIPv6Prefix)
	copy(ip[12:16], sa.Addr[:])

	return ip
}

// sockaddrInet6ToIPAndZone converts a SockaddrInet6 to a net.IP with IPv6 Zone.
// It returns nil if conversion fails.
func sockaddrInet6ToIPAndZone(sa *syscall.SockaddrInet6) (net.IP, string) {
	ip := bsPool.Get(netIPBytesSize)
	copy(ip, sa.Addr[:])

	return ip, ip6ZoneToString(int(sa.ZoneId))
}

// ip6ZoneToString converts an IP6 Zone unix int to a net string
// returns "" if zone is 0.
func ip6ZoneToString(zone int) string {
	if zone == 0 {
		return ""
	}

	if ifi, err := net.InterfaceByIndex(zone); err == nil {
		return ifi.Name
	}

	return int2decimal(uint(zone))
}

// BytesToString converts byte slice to a string without memory allocation.
//
// Note it may break if the implementation of string or slice header changes in the future go versions.
func BytesToString(b []byte) string {
	/* #nosec G103 */
	return *(*string)(unsafe.Pointer(&b))
}

// Convert int to decimal string.
func int2decimal(value uint) string {
	if value == 0 {
		return "0"
	}

	// Assemble decimal in reverse order.
	byteSlice := bsPool.Get(int32size)
	bytesSliceLen := len(byteSlice)

	for ; value > 0; value /= 10 {
		bytesSliceLen--
		byteSlice[bytesSliceLen] = byte(value%decimalBase) + '0'
	}

	return BytesToString(byteSlice[bytesSliceLen:])
}
