package gain

const (
	acceptDataFlag uint64 = 1 << (uint64Size - 1 - iota)
	readDataFlag
	writeDataFlag
	addConnFlag
	closeConnFlag
)

const allFlagsMask = acceptDataFlag | readDataFlag | writeDataFlag | addConnFlag |
	closeConnFlag

func flagToString(flag uint64) string {
	switch {
	case flag&acceptDataFlag > 0:
		return "accept flag"
	case flag&readDataFlag > 0:
		return "read flag"
	case flag&writeDataFlag > 0:
		return "write flag"
	case flag&addConnFlag > 0:
		return "add conn flag"
	case flag&closeConnFlag > 0:
		return "close conn flag"
	}
	return "unknown flag"
}
