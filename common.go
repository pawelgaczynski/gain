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
	"unsafe"

	"golang.org/x/sys/unix"
)

type void struct{}

var member void

const uint64Size = 64

func createClientAddr() (uintptr, uint64) {
	var clientLen = new(uint32)
	clientAddr := &unix.RawSockaddrAny{}
	*clientLen = unix.SizeofSockaddrAny
	return uintptr(unsafe.Pointer(clientAddr)), uint64(uintptr(unsafe.Pointer(clientLen)))
}
