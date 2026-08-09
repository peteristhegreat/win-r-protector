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
	maskCount := 0
	hook := &Hook{
		attempts: make(chan struct{}, 1),
		maskWinPress: func() bool {
			maskCount++
			return true
		},
	}

	if hook.processKey(vkR, wmKeyDown) {
		t.Fatal("R without Windows key was suppressed")
	}
	if hook.processKey(vkLWin, wmKeyDown) {
		t.Fatal("Windows key was suppressed")
	}
	if !hook.processKey(vkR, wmKeyDown) {
		t.Fatal("Win+R keydown was not suppressed")
	}
	if maskCount != 1 {
		t.Fatalf("Windows-key press mask count = %d, want 1", maskCount)
	}
	select {
	case <-hook.attempts:
	default:
		t.Fatal("Win+R attempt was not reported")
	}
	if !hook.processKey(vkR, wmKeyDown) {
		t.Fatal("repeated Win+R keydown was not suppressed")
	}
	if maskCount != 1 {
		t.Fatalf("key repeat changed Windows-key press mask count to %d", maskCount)
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

func TestWindowsKeyMaskIsLimitedToAttemptedChord(t *testing.T) {
	maskCount := 0
	hook := &Hook{
		attempts: make(chan struct{}, 1),
		maskWinPress: func() bool {
			maskCount++
			return true
		},
	}

	if hook.processKey(vkLWin, wmKeyDown) || hook.processKey(vkLWin, wmKeyUp) {
		t.Fatal("standalone Windows key was suppressed")
	}
	if maskCount != 0 {
		t.Fatalf("standalone Windows key was masked %d times", maskCount)
	}

	if hook.processKey(vkRWin, wmKeyDown) {
		t.Fatal("right Windows key was suppressed")
	}
	if !hook.processKey(vkR, wmKeyDown) {
		t.Fatal("right Win+R keydown was not suppressed")
	}
	if !hook.processKey(vkR, wmKeyUp) {
		t.Fatal("right Win+R keyup was not suppressed")
	}
	if hook.processKey(vkRWin, wmKeyUp) {
		t.Fatal("right Windows keyup was suppressed")
	}
	if maskCount != 1 {
		t.Fatalf("right Win+R mask count = %d, want 1", maskCount)
	}

	if hook.processKey(vkLWin, wmKeyDown) {
		t.Fatal("second Windows-key press was suppressed")
	}
	if !hook.processKey(vkR, wmKeyDown) {
		t.Fatal("second Win+R keydown was not suppressed")
	}
	if maskCount != 2 {
		t.Fatalf("new Windows-key press mask count = %d, want 2", maskCount)
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
