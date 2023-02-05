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
)

type buffer struct {
	data   []byte
	iovecs []syscall.Iovec
}

func createBuffer(bufferSize uint) *buffer {
	bufferMem := make([]byte, bufferSize)
	iovec := syscall.Iovec{
		Base: &bufferMem[0],
		Len:  uint64(bufferSize),
	}
	buff := &buffer{
		data:   bufferMem,
		iovecs: []syscall.Iovec{iovec},
	}
	return buff
}

func createBuffers(bufferSize uint, maxConn uint) []*buffer {
	memSize := maxConn * bufferSize
	bufferMem := make([]byte, memSize)
	buffs := make([]*buffer, maxConn)
	for index := range buffs {
		startIndex := index * int(bufferSize)
		buff := bufferMem[startIndex : startIndex+int(bufferSize)]
		iovec := syscall.Iovec{
			Base: &buff[0],
			Len:  uint64(bufferSize),
		}
		buffs[index] = &buffer{
			data:   buff,
			iovecs: []syscall.Iovec{iovec},
		}
	}
	return buffs
}
