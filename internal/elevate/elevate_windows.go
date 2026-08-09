//go:build windows

package elevate

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

type Action string

const (
	Install   Action = "install"
	Start     Action = "start"
	Stop      Action = "stop"
	Restart   Action = "restart"
	Uninstall Action = "uninstall"
)

const seeMaskNoCloseProcess = 0x00000040

var shellExecuteEx = windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteExW")

type shellExecuteInfo struct {
	Size          uint32
	Mask          uint32
	Window        windows.Handle
	Verb          *uint16
	File          *uint16
	Parameters    *uint16
	Directory     *uint16
	Show          int32
	Instance      windows.Handle
	IDList        unsafe.Pointer
	Class         *uint16
	ClassKey      windows.Handle
	HotKey        uint32
	IconOrMonitor windows.Handle
	Process       windows.Handle
}

// Run launches a short-lived elevated copy and waits for its exit code.
func Run(action Action) (uint32, error) {
	executable, err := os.Executable()
	if err != nil {
		return 0, err
	}
	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString(executable)
	parameters, _ := windows.UTF16PtrFromString("--internal-action=" + string(action))
	directory, _ := windows.UTF16PtrFromString(filepath.Dir(executable))

	info := shellExecuteInfo{
		Size:       uint32(unsafe.Sizeof(shellExecuteInfo{})),
		Mask:       seeMaskNoCloseProcess,
		Verb:       verb,
		File:       file,
		Parameters: parameters,
		Directory:  directory,
		Show:       1,
	}

	result, _, callErr := shellExecuteEx.Call(uintptr(unsafe.Pointer(&info)))
	runtime.KeepAlive(verb)
	runtime.KeepAlive(file)
	runtime.KeepAlive(parameters)
	runtime.KeepAlive(directory)
	if result == 0 {
		return 0, fmt.Errorf("request elevation: %w", callErr)
	}
	if info.Process == 0 {
		return 0, fmt.Errorf("elevated process did not return a process handle")
	}
	defer windows.CloseHandle(info.Process)

	if _, err := windows.WaitForSingleObject(info.Process, windows.INFINITE); err != nil {
		return 0, fmt.Errorf("wait for elevated process: %w", err)
	}
	var exitCode uint32
	if err := windows.GetExitCodeProcess(info.Process, &exitCode); err != nil {
		return 0, fmt.Errorf("read elevated process result: %w", err)
	}
	return exitCode, nil
}
