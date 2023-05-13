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
	"net"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"

	"github.com/pawelgaczynski/gain"
)

func int8ToStr(arr []int8) string {
	buffer := make([]byte, 0, len(arr))

	for _, v := range arr {
		if v == 0x00 {
			break
		}

		buffer = append(buffer, byte(v))
	}

	return string(buffer)
}

func checkKernelCompatibility(expectedKernelVersion, expectedMajorVersion int) bool {
	var uname syscall.Utsname

	err := syscall.Uname(&uname)
	if err != nil {
		return false
	}

	kernelString := int8ToStr(uname.Release[:])

	kernelParts := strings.Split(kernelString, ".")
	if len(kernelParts) < 2 {
		return false
	}

	kernelVersion, err := strconv.Atoi(kernelParts[0])
	if err != nil {
		return false
	}

	majorVersion, err := strconv.Atoi(kernelParts[1])
	if err != nil {
		return false
	}

	if expectedKernelVersion < kernelVersion {
		return true
	}

	if expectedMajorVersion < majorVersion {
		return true
	}

	return false
}

var port int32 = 9000

func getTestPort() int {
	return int(atomic.AddInt32(&port, 1))
}

type onStartCallback func(server gain.Server)

type onAcceptCallback func(c gain.Conn)

type onReadCallback func(c gain.Conn, n int)

type onWriteCallback func(c gain.Conn, n int)

type onCloseCallback func(c gain.Conn, err error)

func getFdFromConn(c net.Conn) int {
	v := reflect.Indirect(reflect.ValueOf(c))
	conn := v.FieldByName("conn")
	netFD := reflect.Indirect(conn.FieldByName("fd"))
	pfd := netFD.FieldByName("pfd")
	fd := int(pfd.FieldByName("Sysfd").Int())

	return fd
}
