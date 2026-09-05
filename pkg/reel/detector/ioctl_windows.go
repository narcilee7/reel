//go:build windows

package detector

import "errors"

// winSizeOf is unsupported on windows and returns zero values.
func winSizeOf(fd uintptr) (rows, cols, pxW, pxH int, err error) {
	return 0, 0, 0, 0, errors.New("detector: window size detection not supported on windows")
}
