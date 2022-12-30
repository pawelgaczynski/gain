package gain

import (
	"syscall"
	"unsafe"

	"golang.org/x/net/bpf"
	"golang.org/x/sys/unix"
)

const skfAdOffPlusKSkfAdCPU = 4294963236
const cpuIDSize = 4

type filter []bpf.Instruction

func (f filter) applyTo(fd int) error {
	var err error
	var assembled []bpf.RawInstruction
	if assembled, err = bpf.Assemble(f); err != nil {
		return err
	}
	var program = unix.SockFprog{
		Len:    uint16(len(assembled)),
		Filter: (*unix.SockFilter)(unsafe.Pointer(&assembled[0])),
	}
	var b = (*[unix.SizeofSockFprog]byte)(unsafe.Pointer(&program))[:unix.SizeofSockFprog]
	if _, _, errno := syscall.Syscall6(syscall.SYS_SETSOCKOPT,
		uintptr(fd), uintptr(syscall.SOL_SOCKET), uintptr(unix.SO_ATTACH_REUSEPORT_CBPF),
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
// FIXME: here should be number of cpus instead of workers???
func newFilter(workers uint32) filter {
	return filter{
		bpf.LoadAbsolute{Off: skfAdOffPlusKSkfAdCPU, Size: cpuIDSize},
		bpf.ALUOpConstant{Op: bpf.ALUOpMod, Val: workers},
		bpf.RetA{}, // return 0xffff bytes (or less) from packet
	}
}
