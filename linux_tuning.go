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
