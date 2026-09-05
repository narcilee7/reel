package detector

import (
	"errors"
	"io"
	"time"
)

// ErrNotImplemented is returned by detection layers that are not implemented yet.
var ErrNotImplemented = errors.New("detector: not implemented")

// da1Query is the Device Attributes query sequence (DA1).
const da1Query = "\x1b[c"

// queryDA1 sends a DA1 query and reads the terminal response within the timeout.
//
// TODO: implement the non-blocking read with timeout; reserved for L3
// arbitration when L1 and L2 detection results conflict.
func queryDA1(out io.Writer, in io.Reader, timeout time.Duration) ([]byte, error) {
	return nil, ErrNotImplemented
}
