//go:build windows

// Package keyboardhook owns the per-user low-level keyboard hook used by the
// tray process. It intentionally has no UI dependencies.
package keyboardhook

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	whKeyboardLL = 13
	hcAction     = 0

	wmKeyDown    = 0x0100
	wmKeyUp      = 0x0101
	wmSysKeyDown = 0x0104
	wmSysKeyUp   = 0x0105
	wmQuit       = 0x0012

	vkR    = 0x52
	vkLWin = 0x5B
	vkRWin = 0x5C

	pmNoRemove = 0x0000
)

var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	kernel32                = windows.NewLazySystemDLL("kernel32.dll")
	procCallNextHookEx      = user32.NewProc("CallNextHookEx")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procPeekMessageW        = user32.NewProc("PeekMessageW")
	procPostThreadMessageW  = user32.NewProc("PostThreadMessageW")
	procSetWindowsHookExW   = user32.NewProc("SetWindowsHookExW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	procGetModuleHandleW    = kernel32.NewProc("GetModuleHandleW")
)

// These definitions use uintptr for every pointer-sized Win32 field. This is
// required for the x64 ABI and also keeps the package valid on 32-bit Windows.
type keyboardEvent struct {
	VKCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DWExtraInfo uintptr
}

type point struct {
	X int32
	Y int32
}

type message struct {
	Window  windows.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Point   point
	Private uint32
}

// Hook suppresses Win+R and reports each distinct attempt through Attempts.
type Hook struct {
	attempts chan struct{}
	finished chan error
	stopOnce sync.Once
	stopErr  error

	threadID uint32
	handle   windows.Handle
	callback uintptr

	leftWinDown  bool
	rightWinDown bool
	blockingR    bool
}

// Start installs a global low-level keyboard hook on a dedicated OS thread.
// It does not return until hook installation has succeeded or failed.
func Start() (*Hook, error) {
	hook := &Hook{
		attempts: make(chan struct{}, 1),
		finished: make(chan error, 1),
	}
	ready := make(chan error, 1)
	go hook.run(ready)

	if err := <-ready; err != nil {
		return nil, err
	}
	return hook, nil
}

// Attempts receives one notification for each Win+R press. Notifications are
// coalesced when the UI is already busy handling an attempt.
func (hook *Hook) Attempts() <-chan struct{} {
	return hook.attempts
}

// Close removes the hook and waits for its message-loop thread to finish.
func (hook *Hook) Close() error {
	if hook == nil {
		return nil
	}
	hook.stopOnce.Do(func() {
		result, _, callErr := procPostThreadMessageW.Call(uintptr(hook.threadID), wmQuit, 0, 0)
		if result == 0 {
			hook.stopErr = win32Error("post keyboard hook shutdown message", callErr)
			return
		}
		hook.stopErr = <-hook.finished
	})
	return hook.stopErr
}

func (hook *Hook) run(ready chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hook.threadID = windows.GetCurrentThreadId()

	// Force creation of this thread's message queue before another goroutine
	// can call PostThreadMessageW during shutdown.
	var msg message
	procPeekMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, pmNoRemove)

	hook.callback = windows.NewCallback(hook.callbackProc)
	module, _, moduleErr := procGetModuleHandleW.Call(0)
	if module == 0 {
		err := win32Error("get application module handle", moduleErr)
		ready <- err
		hook.finished <- err
		return
	}

	handle, _, hookErr := procSetWindowsHookExW.Call(whKeyboardLL, hook.callback, module, 0)
	if handle == 0 {
		err := win32Error("install low-level keyboard hook", hookErr)
		ready <- err
		hook.finished <- err
		return
	}
	hook.handle = windows.Handle(handle)
	ready <- nil

	var loopErr error
	for {
		result, _, callErr := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		switch int32(result) {
		case -1:
			loopErr = win32Error("read keyboard hook message", callErr)
			goto stopped
		case 0:
			goto stopped
		default:
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
		}
	}

stopped:
	result, _, callErr := procUnhookWindowsHookEx.Call(uintptr(hook.handle))
	if result == 0 && loopErr == nil {
		loopErr = win32Error("remove low-level keyboard hook", callErr)
	}
	hook.handle = 0
	hook.finished <- loopErr
}

func (hook *Hook) callbackProc(code int32, wParam uintptr, event *keyboardEvent) uintptr {
	lParam := uintptr(unsafe.Pointer(event))
	if code != hcAction || event == nil {
		return callNextHook(hook.handle, code, wParam, lParam)
	}

	if hook.processKey(event.VKCode, uint32(wParam)) {
		return 1
	}
	return callNextHook(hook.handle, code, wParam, lParam)
}

func (hook *Hook) processKey(vkCode, event uint32) bool {
	keyDown := event == wmKeyDown || event == wmSysKeyDown
	keyUp := event == wmKeyUp || event == wmSysKeyUp

	switch vkCode {
	case vkLWin:
		if keyDown {
			hook.leftWinDown = true
		} else if keyUp {
			hook.leftWinDown = false
		}
	case vkRWin:
		if keyDown {
			hook.rightWinDown = true
		} else if keyUp {
			hook.rightWinDown = false
		}
	case vkR:
		if keyDown && (hook.leftWinDown || hook.rightWinDown) {
			if !hook.blockingR {
				hook.blockingR = true
				select {
				case hook.attempts <- struct{}{}:
				default:
				}
			}
			return true
		}
		if keyUp && hook.blockingR {
			hook.blockingR = false
			return true
		}
	}

	return false
}

func callNextHook(handle windows.Handle, code int32, wParam, lParam uintptr) uintptr {
	result, _, _ := procCallNextHookEx.Call(uintptr(handle), uintptr(code), wParam, lParam)
	return result
}

func win32Error(operation string, err error) error {
	if err == nil || errors.Is(err, windows.ERROR_SUCCESS) {
		return fmt.Errorf("%s", operation)
	}
	return fmt.Errorf("%s: %w", operation, err)
}
