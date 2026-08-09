//go:build windows

package startup

import (
	"fmt"
	"os"

	"github.com/phyatt/win-r-protector/internal/appmeta"
	"golang.org/x/sys/windows/registry"
)

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`

func Register() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open logon startup key: %w", err)
	}
	defer key.Close()
	if err := key.SetStringValue(appmeta.StartupValueName, `"`+executable+`"`); err != nil {
		return fmt.Errorf("register logon startup: %w", err)
	}
	return nil
}

func Unregister() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open logon startup key: %w", err)
	}
	defer key.Close()
	if err := key.DeleteValue(appmeta.StartupValueName); err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("remove logon startup: %w", err)
	}
	return nil
}
