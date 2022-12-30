package gain

import (
	"errors"
)

var (
	errNotImplemented = errors.New("not implemented")
	errNotSupported   = errors.New("not supported")
	errSkippable      = errors.New("skippable")
)
