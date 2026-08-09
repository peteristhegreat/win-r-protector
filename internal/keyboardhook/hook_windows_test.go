//go:build windows

package keyboardhook

import (
	"runtime"
	"testing"
	"unsafe"
)

func TestNativeStructureSizes(t *testing.T) {
	wantKeyboardEvent := uintptr(20)
	wantMessage := uintptr(32)
	if runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64" {
		wantKeyboardEvent = 24
		wantMessage = 48
	}

	if got := unsafe.Sizeof(keyboardEvent{}); got != wantKeyboardEvent {
		t.Fatalf("keyboard event size = %d, want %d for %s", got, wantKeyboardEvent, runtime.GOARCH)
	}
	if got := unsafe.Sizeof(message{}); got != wantMessage {
		t.Fatalf("message size = %d, want %d for %s", got, wantMessage, runtime.GOARCH)
	}
}

func TestWinRDetectionAndSuppression(t *testing.T) {
	hook := &Hook{attempts: make(chan struct{}, 1)}

	if hook.processKey(vkR, wmKeyDown) {
		t.Fatal("R without Windows key was suppressed")
	}
	if hook.processKey(vkLWin, wmKeyDown) {
		t.Fatal("Windows key was suppressed")
	}
	if !hook.processKey(vkR, wmKeyDown) {
		t.Fatal("Win+R keydown was not suppressed")
	}
	select {
	case <-hook.attempts:
	default:
		t.Fatal("Win+R attempt was not reported")
	}
	if !hook.processKey(vkR, wmKeyDown) {
		t.Fatal("repeated Win+R keydown was not suppressed")
	}
	select {
	case <-hook.attempts:
		t.Fatal("key repeat produced another attempt")
	default:
	}
	if !hook.processKey(vkR, wmKeyUp) {
		t.Fatal("Win+R keyup was not suppressed")
	}
	if hook.processKey(vkLWin, wmKeyUp) {
		t.Fatal("Windows keyup was suppressed")
	}
}

func TestHookLifecycle(t *testing.T) {
	hook, err := Start()
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := hook.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
