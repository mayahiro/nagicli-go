//go:build linux && (amd64 || arm64)

package prompt

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

type terminalState = syscall.Termios

func (unixTerminalAccess) getState(fd int) (terminalState, error) {
	state := terminalState{}
	if err := ioctlTerminalState(fd, syscall.TCGETS, &state); err != nil {
		return terminalState{}, err
	}
	return state, nil
}

func (unixTerminalAccess) setState(fd int, state *terminalState) error {
	return ioctlTerminalState(fd, syscall.TCSETS, state)
}

func ioctlTerminalState(fd int, request uintptr, state *terminalState) error {
	_, _, callErr := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		request,
		uintptr(unsafe.Pointer(state)),
	)
	if callErr != 0 {
		return callErr
	}
	return nil
}

func (unixTerminalAccess) waitReadable(fd int, timeout time.Duration) (bool, error) {
	var readSet syscall.FdSet
	capacity := len(readSet.Bits) * 64
	if fd < 0 || fd >= capacity {
		return false, fmt.Errorf("prompt descriptor %d exceeds select capacity %d", fd, capacity)
	}
	readSet.Bits[fd/64] |= int64(1) << uint(fd%64)
	timeval := syscall.NsecToTimeval(timeout.Nanoseconds())
	ready, err := syscall.Select(fd+1, &readSet, nil, nil, &timeval)
	if err != nil {
		return false, err
	}
	return ready > 0 && readSet.Bits[fd/64]&(int64(1)<<uint(fd%64)) != 0, nil
}
