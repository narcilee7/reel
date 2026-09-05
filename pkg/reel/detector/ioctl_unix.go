//go:build !windows

package detector

import (
	"syscall"
	"unsafe"
)

type winSize struct {
	Row, Col       uint16
	Xpixel, Ypixel uint16
}

// winSizeOf returns window dimensions in cells and pixels for the given fd.
func winSizeOf(fd uintptr) (rows, cols, pxW, pxH int, err error) {
	var ws winSize
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&ws)))
	if errno != 0 {
		return 0, 0, 0, 0, errno
	}
	return int(ws.Row), int(ws.Col), int(ws.Xpixel), int(ws.Ypixel), nil
}
