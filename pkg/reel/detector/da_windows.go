//go:build windows

package detector

import (
	"errors"
	"time"
)

// ErrNotImplemented is returned by detection layers that are not implemented
// on the current platform.
var ErrNotImplemented = errors.New("detector: not implemented")

// DAResult holds the parsed responses of a DA1/DA2 query round.
type DAResult struct {
	DA1Params []int
	DA2       [3]int
}

// queryDeviceAttributes is not supported on Windows; Phase 2 detection on
// Windows stays at layers L1/L2.
func queryDeviceAttributes(timeout time.Duration) (*DAResult, error) {
	return nil, ErrNotImplemented
}
