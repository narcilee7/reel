//go:build !windows && !linux && !darwin

package detector

import "golang.org/x/sys/unix"

func getTermios(fd int) (*unix.Termios, error) {
	return nil, ErrNotImplemented
}

func setTermios(fd int, t *unix.Termios) error {
	return ErrNotImplemented
}
