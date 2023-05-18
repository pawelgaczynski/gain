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

package virtualmem

import (
	"fmt"
	"os"
	"runtime"
	"testing"

	. "github.com/stretchr/testify/require"
)

func TestVirtualMemPool(t *testing.T) {
	pool := NewPool()

	size := os.Getpagesize()
	vm := NewVirtualMem(size)

	pool.Put(vm)

	vmFromPool := pool.Get(size)

	Same(t, vm, vmFromPool)
	runtime.KeepAlive(vm)

	vm = Get(size)

	Put(vm)

	vmFromPool = Get(size)

	Same(t, vm, vmFromPool)
	runtime.KeepAlive(vm)

	Nil(t, pool.Get(0))
	Nil(t, pool.Get(-1))

	vm = NewVirtualMem(maxVMSize + pageSize)

	pool.Put(vm)

	vmFromPool = pool.Get(maxVMSize + pageSize)

	NotEqual(t, fmt.Sprintf("%p\n", vmFromPool), fmt.Sprintf("%p\n", vm))

	runtime.KeepAlive(vm)
}
