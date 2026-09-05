//go:build !windows

package detector

import (
	"bytes"
	"errors"
	"os"
	"strconv"
	"time"

	"golang.org/x/sys/unix"
)

// ErrNotImplemented is returned by detection layers that are not implemented
// on the current platform.
var ErrNotImplemented = errors.New("detector: not implemented")

// daQueries sends DA1 and DA2 back to back; responses arrive in order.
const daQueries = "\x1b[c\x1b[>c"

// DAResult holds the parsed responses of a DA1/DA2 query round.
type DAResult struct {
	// DA1Params are the DA1 response parameters (e.g. [62, 4, 22]);
	// parameter 4 reports sixel support.
	DA1Params []int
	// DA2 holds the three DA2 parameters (terminal type code, version,
	// level). Absent parameters are -1.
	DA2 [3]int
	// Raw1/Raw2 hold the unmodified response bytes for diagnostics
	// (fingerprint calibration). Nil for a response that never arrived.
	Raw1, Raw2 []byte
}

// queryDeviceAttributes opens /dev/tty and runs one DA1+DA2 query round.
// The terminal is switched to raw mode for the duration of the read and
// restored afterwards, so response bytes can never leak into the render
// stream. Returns an error when no tty is available or the query times out;
// callers treat that as "keep the heuristic result", not as a failure.
func queryDeviceAttributes(timeout time.Duration) (*DAResult, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	defer tty.Close()
	return queryDAOnTTY(tty, timeout)
}

// queryDAOnTTY sends the DA queries on tty and reads both responses.
func queryDAOnTTY(tty *os.File, timeout time.Duration) (*DAResult, error) {
	fd := int(tty.Fd())
	old, err := getTermios(fd)
	if err != nil {
		return nil, err
	}
	raw := *old
	raw.Iflag &^= unix.IXON | unix.ICRNL | unix.BRKINT | unix.INPCK | unix.ISTRIP
	raw.Lflag &^= unix.ECHO | unix.ICANON | unix.ISIG | unix.IEXTEN
	raw.Oflag &^= unix.OPOST
	raw.Cc[unix.VMIN] = 1
	raw.Cc[unix.VTIME] = 0
	if err := setTermios(fd, &raw); err != nil {
		return nil, err
	}
	defer setTermios(fd, old)

	if _, err := tty.WriteString(daQueries); err != nil {
		return nil, err
	}

	da1, da2, err := readDAResponses(tty, timeout)
	if err != nil {
		return nil, err
	}
	res := &DAResult{DA2: [3]int{-1, -1, -1}}
	if da1 != nil {
		res.Raw1 = da1
		res.DA1Params = parseDA1(da1)
	}
	if da2 != nil {
		res.Raw2 = da2
		res.DA2 = parseDA2(da2)
	}
	return res, nil
}

// readDAResponses reads up to two DA responses, each terminated by 'c'. It
// returns as soon as both arrive or the deadline passes; a nil slice marks a
// response that never arrived.
func readDAResponses(tty *os.File, timeout time.Duration) (da1, da2 []byte, err error) {
	if err := tty.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return nil, nil, err
	}
	buf := make([]byte, 0, 64)
	one := make([]byte, 1)
	for da1 == nil || da2 == nil {
		n, err := tty.Read(one)
		if err != nil || n == 0 {
			break // deadline or EOF: respond with whatever arrived
		}
		buf = append(buf, one[0])
		if one[0] != 'c' {
			if len(buf) > 128 { // desync guard
				buf = buf[:0]
			}
			continue
		}
		switch {
		case bytes.Contains(buf, []byte("?")) && da1 == nil:
			da1 = append([]byte(nil), buf...)
		case bytes.Contains(buf, []byte(">")) && da2 == nil:
			da2 = append([]byte(nil), buf...)
		}
		buf = buf[:0]
	}
	if da1 == nil && da2 == nil {
		return nil, nil, ErrNotImplemented
	}
	return da1, da2, nil
}

// parseDA1 parses a DA1 response (CSI ? Ps;Ps;... c) into its parameters.
func parseDA1(resp []byte) []int {
	i := bytes.IndexByte(resp, '?')
	if i < 0 {
		return nil
	}
	return parseDAParams(resp[i+1:], 'c')
}

// parseDA2 parses a DA2 response (CSI > Ps;Ps;Ps c) into its three
// parameters; absent parameters are -1.
func parseDA2(resp []byte) [3]int {
	out := [3]int{-1, -1, -1}
	i := bytes.IndexByte(resp, '>')
	if i < 0 {
		return out
	}
	params := parseDAParams(resp[i+1:], 'c')
	for j := 0; j < len(params) && j < 3; j++ {
		out[j] = params[j]
	}
	return out
}

// parseDAParams splits the bytes before end on ';' and parses the numeric
// fields, skipping empty and non-numeric ones.
func parseDAParams(b []byte, end byte) []int {
	if j := bytes.IndexByte(b, end); j >= 0 {
		b = b[:j]
	}
	fields := bytes.Split(b, []byte(";"))
	out := make([]int, 0, len(fields))
	for _, f := range fields {
		f = bytes.TrimSpace(f)
		if len(f) == 0 {
			continue
		}
		n, err := strconv.Atoi(string(f))
		if err != nil {
			continue
		}
		out = append(out, n)
	}
	return out
}
