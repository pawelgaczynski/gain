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
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/net/bpf"
	"golang.org/x/sys/unix"
)

const (
	skfAdOffPlusKSkfAdCPU = 4294963236
	cpuIDSize             = 4
)

type filter []bpf.Instruction

func (f filter) applyTo(fileDescriptor int) error {
	var (
		err       error
		assembled []bpf.RawInstruction
	)

	if assembled, err = bpf.Assemble(f); err != nil {
		return fmt.Errorf("BPF filter assemble error: %w", err)
	}
	program := unix.SockFprog{
		Len:    uint16(len(assembled)),
		Filter: (*unix.SockFilter)(unsafe.Pointer(&assembled[0])),
	}
	b := (*[unix.SizeofSockFprog]byte)(unsafe.Pointer(&program))[:unix.SizeofSockFprog]

	if _, _, errno := syscall.Syscall6(syscall.SYS_SETSOCKOPT,
		uintptr(fileDescriptor), uintptr(syscall.SOL_SOCKET), uintptr(unix.SO_ATTACH_REUSEPORT_CBPF),
		uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), 0); errno != 0 {
		return errno
	}

	return nil
}

// /* A = raw_smp_processor_id(). */
// { BPF_LD  | BPF_W | BPF_ABS, 0, 0, SKF_AD_OFF + SKF_AD_CPU },
// /* Adjust the CPUID to socket group size. */
// { BPF_ALU | BPF_MOD | BPF_K, 0, 0, sock_count },
// /* Return A. */
// { BPF_RET | BPF_A, 0, 0, 0 },
//
//nolint:godot
func newFilter(cpus uint32) filter {
	return filter{
		bpf.LoadAbsolute{Off: skfAdOffPlusKSkfAdCPU, Size: cpuIDSize},
		bpf.ALUOpConstant{Op: bpf.ALUOpMod, Val: cpus},
		bpf.RetA{}, // return 0xffff bytes (or less) from packet.
	}
}
