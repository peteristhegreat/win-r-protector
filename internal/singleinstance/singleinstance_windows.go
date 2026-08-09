//go:build windows

package singleinstance

import (
	"errors"
	"fmt"

	"github.com/phyatt/win-r-protector/internal/appmeta"
	"golang.org/x/sys/windows"
)

type Lock struct {
	handle windows.Handle
}

func Acquire() (*Lock, bool, error) {
	name, err := windows.UTF16PtrFromString(appmeta.TrayMutexName)
	if err != nil {
		return nil, false, err
	}
	handle, err := windows.CreateMutex(nil, false, name)
	if handle == 0 {
		return nil, false, fmt.Errorf("create tray mutex: %w", err)
	}
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		windows.CloseHandle(handle)
		return nil, false, nil
	}
	if err != nil {
		windows.CloseHandle(handle)
		return nil, false, fmt.Errorf("create tray mutex: %w", err)
	}
	return &Lock{handle: handle}, true, nil
}

func (lock *Lock) Close() {
	if lock != nil && lock.handle != 0 {
		windows.CloseHandle(lock.handle)
		lock.handle = 0
	}
}
