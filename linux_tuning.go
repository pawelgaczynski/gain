package gain

import (
	"fmt"
	"os"
	"runtime"
	"syscall"

	"golang.org/x/sys/unix"
)

func setProcessPriority() error {
	pid := os.Getpid()
	return syscall.Setpriority(syscall.PRIO_PROCESS, pid, -19)
}

func setAffinity(index int) error {
	var newMask unix.CPUSet
	newMask.Zero()
	cpuIndex := (index) % (runtime.NumCPU())
	newMask.Set(cpuIndex)
	err := unix.SchedSetaffinity(0, &newMask)
	if err != nil {
		return fmt.Errorf("SchedSetaffinity: %w, %v", err, newMask)
	}
	return nil
}
