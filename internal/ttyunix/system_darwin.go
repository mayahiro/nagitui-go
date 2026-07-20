//go:build darwin && (amd64 || arm64)

package ttyunix

import (
	"errors"
	"syscall"
	"time"
	"unsafe"
)

type terminalState = syscall.Termios

func (unixBackend) getState(fd int) (terminalState, error) {
	state := terminalState{}
	if err := ioctlTerminalState(fd, syscall.TIOCGETA, &state); err != nil {
		return terminalState{}, err
	}
	return state, nil
}

func (unixBackend) setState(fd int, state *terminalState) error {
	return ioctlTerminalState(fd, syscall.TIOCSETA, state)
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

func (unixBackend) waitReadable(fd int, timeout time.Duration) (bool, error) {
	var readSet syscall.FdSet
	if err := validateSelectFD(fd, len(readSet.Bits)*32); err != nil {
		return false, err
	}
	readSet.Bits[fd/32] |= int32(1) << uint(fd%32)
	if err := syscall.Select(fd+1, &readSet, nil, nil, selectTimeout(timeout)); err != nil {
		if errors.Is(err, syscall.EINTR) {
			return false, nil
		}
		return false, err
	}
	return readSet.Bits[fd/32]&(int32(1)<<uint(fd%32)) != 0, nil
}
