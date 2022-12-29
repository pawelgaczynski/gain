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
	var flags uint32
	var result string
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
	var flags uint32
	var result bool
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
