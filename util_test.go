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
	"sync/atomic"

	"github.com/pawelgaczynski/gain"
)

var port int32 = 9000

func getTestPort() int {
	return int(atomic.AddInt32(&port, 1))
}

type onStartCallback func(server *gain.Server)

type onOpenCallback func(c gain.Conn)

type onReadCallback func(c gain.Conn)

type onWriteCallback func(c gain.Conn)

type onCloseCallback func(c gain.Conn)

func getFdFromConn(c net.Conn) int {
	v := reflect.Indirect(reflect.ValueOf(c))
	conn := v.FieldByName("conn")
	netFD := reflect.Indirect(conn.FieldByName("fd"))
	pfd := netFD.FieldByName("pfd")
	fd := int(pfd.FieldByName("Sysfd").Int())

	return fd
}
