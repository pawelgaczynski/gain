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

package iouring

import "fmt"

const (
	probeOpsSize = int(OpLast + 1)
)

type (
	Probe struct {
		LastOp uint8
		OpsLen uint8
		Res    uint16
		Res2   [3]uint32
		Ops    [probeOpsSize]probeOp
	}
	probeOp struct {
		Op    uint8
		Res   uint8
		Flags uint16
		Res2  uint32
	}
)

func (p Probe) IsSupported(op uint8) bool {
	for i := uint8(0); i < p.OpsLen; i++ {
		if p.Ops[i].Op != op {
			continue
		}

		return p.Ops[i].Flags&opSupported > 0
	}

	return false
}

func CheckAvailableFeatures() (string, error) {
	ring := newRing()

	var (
		flags  uint32
		result string
	)

	err := ring.QueueInit(1, flags)
	if err != nil {
		return result, err
	}

	var probe *Probe

	probe, err = ring.RegisterProbe()
	if err != nil {
		return result, err
	}

	err = ring.QueueExit()
	if err != nil {
		return result, err
	}

	for opCode, opCodeName := range opCodesMap {
		var status string
		if !probe.IsSupported(opCode) {
			status = " NOT"
		}
		result += fmt.Sprintf("%s is%s supported\n", opCodeName, status)
	}

	return result, nil
}

func IsOpSupported(opCode uint8) (bool, error) {
	ring := newRing()

	var (
		flags  uint32
		result bool
	)

	err := ring.QueueInit(1, flags)
	if err != nil {
		return result, err
	}

	var probe *Probe

	probe, err = ring.RegisterProbe()
	if err != nil {
		return result, err
	}

	err = ring.QueueExit()
	if err != nil {
		return result, err
	}

	return probe.IsSupported(opCode), nil
}
