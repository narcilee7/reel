//go:build !windows

package detector

import (
	"bytes"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// verifyTimeout bounds each L5 verification round trip.
const verifyTimeout = 500 * time.Millisecond

// kittyVerifyQuery is a 1x1 pixel kitty graphics query frame (q=1): the
// terminal answers \x1b_Gi=1;OK\x1b\\ when the graphics protocol works.
const kittyVerifyQuery = "\x1b_Gi=1,f=32,s=1,v=1,q=1;AAAA\x1b\\"

// VerifyProfile re-checks the profile's protocol hints with interactive
// terminal queries and drops the hints that fail. Unverifiable hints
// (iTerm2, which has no query mechanism and is trusted via TERM_PROGRAM, and
// AnsiArt/Plain, which need no verification) pass through unchanged.
//
// This implements layer L5 and is only invoked by the Engine when
// REEL_VERIFY_PROTOCOL=1 is set; the default probe path never sends test
// images.
func VerifyProfile(p *TerminalProfile) *TerminalProfile {
	if p == nil {
		return nil
	}
	out := *p
	out.Protocols = make([]ProtocolHint, 0, len(p.Protocols))
	for _, h := range p.Protocols {
		if verifyHint(h) {
			out.Protocols = append(out.Protocols, h)
		}
	}
	return &out
}

func verifyHint(h ProtocolHint) bool {
	switch h.Name {
	case "kitty":
		return verifyKitty(verifyTimeout)
	case "sixel":
		return verifySixel(verifyTimeout)
	default:
		return true
	}
}

// verifyKitty sends the q=1 graphics query and expects an OK response.
func verifyKitty(timeout time.Duration) bool {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return false
	}
	defer tty.Close()

	fd := int(tty.Fd())
	old, err := getTermios(fd)
	if err != nil {
		return false
	}
	raw := *old
	raw.Lflag &^= unix.ECHO | unix.ICANON | unix.ISIG
	raw.Cc[unix.VMIN] = 1
	raw.Cc[unix.VTIME] = 0
	if err := setTermios(fd, &raw); err != nil {
		return false
	}
	defer setTermios(fd, old)

	if _, err := tty.WriteString(kittyVerifyQuery); err != nil {
		return false
	}
	if err := tty.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return false
	}
	buf := make([]byte, 0, 64)
	one := make([]byte, 32)
	for len(buf) < 256 {
		n, err := tty.Read(one)
		if err != nil || n == 0 {
			break
		}
		buf = append(buf, one[:n]...)
		if bytes.Contains(buf, []byte("\x1b\\")) {
			break
		}
	}
	return bytes.Contains(buf, []byte("i=1;OK"))
}

// verifySixel re-runs the DA1 query and checks the sixel parameter (4).
func verifySixel(timeout time.Duration) bool {
	da, err := queryDeviceAttributes(timeout)
	return err == nil && da != nil && hasDAParam(da.DA1Params, 4)
}
