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
	DA1Params  []int
	DA2        [3]int
	Raw1, Raw2 []byte // always nil on Windows (no DA query)
}

// errWindowsDA explains the L3 stub. Detection on Windows stays at layers
// L1/L2/L4; `reel probe` surfaces this message plus the env-based evidence.
var errWindowsDA = errors.New("detector: Windows 动态探测未实现，当前结论来自环境变量；可用 `reel probe` 查看细节")

// queryDeviceAttributes is not supported on Windows; Phase 2 detection on
// Windows stays at layers L1/L2.
func queryDeviceAttributes(timeout time.Duration) (*DAResult, error) {
	return nil, errWindowsDA
}
