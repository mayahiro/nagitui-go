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

func (unixBackend) wait(inputFD, wakeFD int, timeout time.Duration, hasTimeout bool) (waitResult, error) {
	var readSet syscall.FdSet
	capacity := len(readSet.Bits) * 32
	if err := validateSelectFD(inputFD, capacity); err != nil {
		return waitResult{}, err
	}
	if err := validateSelectFD(wakeFD, capacity); err != nil {
		return waitResult{}, err
	}
	readSet.Bits[inputFD/32] |= int32(1) << uint(inputFD%32)
	readSet.Bits[wakeFD/32] |= int32(1) << uint(wakeFD%32)
	if err := syscall.Select(max(inputFD, wakeFD)+1, &readSet, nil, nil, selectTimeout(timeout, hasTimeout)); err != nil {
		if errors.Is(err, syscall.EINTR) {
			return waitResult{}, nil
		}
		return waitResult{}, err
	}
	return waitResult{
		input: readSet.Bits[inputFD/32]&(int32(1)<<uint(inputFD%32)) != 0,
		wake:  readSet.Bits[wakeFD/32]&(int32(1)<<uint(wakeFD%32)) != 0,
	}, nil
}
