//go:build !windows

package detector

import (
	"os"
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

// ReprobeGeometry refreshes the terminal cell and grid size from the OS. It
// is the cheap L4 re-probe used by Engine.Reprobe for TUI resize handling.
func ReprobeGeometry() (cell, grid Size, err error) {
	rows, cols, pxW, pxH, err := winSizeOf(os.Stdout.Fd())
	if err != nil || cols <= 0 {
		return Size{}, Size{}, err
	}
	grid = Size{Width: cols, Height: rows}
	if rows > 0 && pxW > 0 {
		cell = Size{Width: pxW / cols, Height: pxH / rows}
	}
	return cell, grid, nil
}
