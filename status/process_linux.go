//go:build linux && (amd64 || arm64)

package status

import (
	"syscall"
	"unsafe"
)

type windowSize struct {
	rows, columns    uint16
	xPixels, yPixels uint16
}

func (unixTerminalAccess) isTerminal(fd int) bool {
	state := syscall.Termios{}
	return descriptorIoctl(fd, syscall.TCGETS, unsafe.Pointer(&state)) == nil
}

func (unixTerminalAccess) width(fd int) (uint16, error) {
	size := windowSize{}
	if err := descriptorIoctl(fd, syscall.TIOCGWINSZ, unsafe.Pointer(&size)); err != nil {
		return 0, err
	}
	return size.columns, nil
}

func descriptorIoctl(fd int, request uintptr, target unsafe.Pointer) error {
	_, _, callErr := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), request, uintptr(target))
	if callErr != 0 {
		return callErr
	}
	return nil
}
