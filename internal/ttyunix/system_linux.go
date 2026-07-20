//go:build linux && (amd64 || arm64)

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
	if err := ioctlTerminalState(fd, syscall.TCGETS, &state); err != nil {
		return terminalState{}, err
	}
	return state, nil
}

func (unixBackend) setState(fd int, state *terminalState) error {
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

func (unixBackend) waitReadable(fd int, timeout time.Duration) (bool, error) {
	var readSet syscall.FdSet
	if err := validateSelectFD(fd, len(readSet.Bits)*64); err != nil {
		return false, err
	}
	readSet.Bits[fd/64] |= int64(1) << uint(fd%64)
	ready, err := syscall.Select(fd+1, &readSet, nil, nil, selectTimeout(timeout))
	if errors.Is(err, syscall.EINTR) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return ready > 0 && readSet.Bits[fd/64]&(int64(1)<<uint(fd%64)) != 0, nil
}
