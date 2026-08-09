//go:build windows

// Package applog provides a small append-only diagnostic log for failures that
// happen before or outside the tray UI.
package applog

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/phyatt/win-r-protector/internal/appmeta"
)

const filename = "win-r-protector.log"

var state struct {
	sync.RWMutex
	logger *log.Logger
	path   string
	file   *os.File
}

func init() {
	state.logger = log.New(io.Discard, "", log.Ldate|log.Ltime|log.Lmicroseconds)
}

// Start opens %LOCALAPPDATA%\Win-R Protector\win-r-protector.log in append
// mode. Calling Start more than once in a process is harmless.
func Start() (string, error) {
	state.Lock()
	defer state.Unlock()

	if state.file != nil {
		return state.path, nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	state.path = filepath.Join(base, appmeta.Name, filename)
	if err := os.MkdirAll(filepath.Dir(state.path), 0o700); err != nil {
		return state.path, err
	}
	file, err := os.OpenFile(state.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return state.path, err
	}
	state.file = file
	state.logger.SetOutput(file)
	return state.path, nil
}

func Path() string {
	state.RLock()
	defer state.RUnlock()
	if state.file == nil {
		return ""
	}
	return state.path
}

func Errorf(format string, args ...any) {
	state.RLock()
	logger := state.logger
	state.RUnlock()
	logger.Printf("ERROR "+format, args...)
}

func Close() error {
	state.Lock()
	defer state.Unlock()
	if state.file == nil {
		return nil
	}
	err := state.file.Close()
	state.file = nil
	state.logger.SetOutput(io.Discard)
	return err
}
