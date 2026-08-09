//go:build windows

package servicecontrol

import (
	"errors"
	"fmt"
	"os"
	"time"
	"unsafe"

	"github.com/phyatt/win-r-protector/internal/appmeta"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const stateChangeTimeout = 20 * time.Second

type Info struct {
	Installed bool
	State     svc.State
}

func IsServiceProcess() (bool, error) {
	return svc.IsWindowsService()
}

func Run() error {
	return svc.Run(appmeta.ServiceName, handler{})
}

type handler struct{}

func (handler) Execute(_ []string, changes <-chan svc.ChangeRequest, statuses chan<- svc.Status) (bool, uint32) {
	statuses <- svc.Status{State: svc.StartPending}
	status := svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	statuses <- status

	for change := range changes {
		switch change.Cmd {
		case svc.Interrogate:
			statuses <- status
		case svc.Stop, svc.Shutdown:
			statuses <- svc.Status{State: svc.StopPending}
			return false, 0
		}
	}

	return false, 0
}

// Status deliberately requests query-only access so it works from the
// unelevated tray process.
func Status() (Info, error) {
	scm, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return Info{}, fmt.Errorf("connect to Service Control Manager: %w", err)
	}
	defer windows.CloseServiceHandle(scm)

	name, err := windows.UTF16PtrFromString(appmeta.ServiceName)
	if err != nil {
		return Info{}, err
	}
	service, err := windows.OpenService(scm, name, windows.SERVICE_QUERY_STATUS)
	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return Info{Installed: false}, nil
	}
	if err != nil {
		return Info{}, fmt.Errorf("open service: %w", err)
	}
	defer windows.CloseServiceHandle(service)

	var raw windows.SERVICE_STATUS_PROCESS
	var needed uint32
	err = windows.QueryServiceStatusEx(
		service,
		windows.SC_STATUS_PROCESS_INFO,
		(*byte)(unsafe.Pointer(&raw)),
		uint32(unsafe.Sizeof(raw)),
		&needed,
	)
	if err != nil {
		return Info{}, fmt.Errorf("query service: %w", err)
	}

	return Info{Installed: true, State: svc.State(raw.CurrentState)}, nil
}

func StateText(info Info) string {
	if !info.Installed {
		return "Not installed"
	}
	switch info.State {
	case svc.Stopped:
		return "Stopped"
	case svc.StartPending:
		return "Starting"
	case svc.StopPending:
		return "Stopping"
	case svc.Running:
		return "Running"
	case svc.ContinuePending:
		return "Resuming"
	case svc.PausePending:
		return "Pausing"
	case svc.Paused:
		return "Paused"
	default:
		return "Unknown"
	}
}

func Install() error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}

	manager, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to Service Control Manager: %w", err)
	}
	defer manager.Disconnect()

	service, err := manager.CreateService(appmeta.ServiceName, executable, mgr.Config{
		DisplayName:  appmeta.ServiceDisplayName,
		Description:  appmeta.ServiceDescription,
		StartType:    mgr.StartAutomatic,
		ErrorControl: mgr.ErrorNormal,
	})
	if errors.Is(err, windows.ERROR_SERVICE_EXISTS) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	defer service.Close()

	if err := service.Start(); err != nil && !errors.Is(err, windows.ERROR_SERVICE_ALREADY_RUNNING) {
		return fmt.Errorf("start installed service: %w", err)
	}
	return waitForState(service, svc.Running)
}

func Start() error {
	return withService(func(service *mgr.Service) error {
		status, err := service.Query()
		if err != nil {
			return err
		}
		switch status.State {
		case svc.Running:
			return nil
		case svc.StartPending, svc.ContinuePending:
			return waitForState(service, svc.Running)
		case svc.StopPending:
			if err := waitForState(service, svc.Stopped); err != nil {
				return err
			}
		case svc.PausePending:
			if err := waitForState(service, svc.Paused); err != nil {
				return err
			}
			fallthrough
		case svc.Paused:
			if _, err := service.Control(svc.Continue); err != nil {
				return err
			}
			return waitForState(service, svc.Running)
		}
		if err := service.Start(); err != nil && !errors.Is(err, windows.ERROR_SERVICE_ALREADY_RUNNING) {
			return err
		}
		return waitForState(service, svc.Running)
	})
}

// WaitForRunning waits using query-only SCM access, so an already-starting
// service does not cause an unnecessary UAC prompt.
func WaitForRunning() error {
	deadline := time.Now().Add(stateChangeTimeout)
	for time.Now().Before(deadline) {
		info, err := Status()
		if err != nil {
			return err
		}
		if !info.Installed {
			return fmt.Errorf("service is not installed")
		}
		if info.State == svc.Running {
			return nil
		}
		if info.State != svc.StartPending && info.State != svc.ContinuePending {
			return fmt.Errorf("service entered state %s", StateText(info))
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for service to run")
}

func Stop() error {
	return withService(stop)
}

func Restart() error {
	return withService(func(service *mgr.Service) error {
		if err := stop(service); err != nil {
			return err
		}
		if err := service.Start(); err != nil {
			return err
		}
		return waitForState(service, svc.Running)
	})
}

func Uninstall() error {
	manager, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to Service Control Manager: %w", err)
	}
	defer manager.Disconnect()

	service, err := manager.OpenService(appmeta.ServiceName)
	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open service: %w", err)
	}
	defer service.Close()

	if err := stop(service); err != nil {
		return fmt.Errorf("stop service: %w", err)
	}
	if err := service.Delete(); err != nil {
		return fmt.Errorf("delete service: %w", err)
	}
	return nil
}

func withService(fn func(*mgr.Service) error) error {
	manager, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to Service Control Manager: %w", err)
	}
	defer manager.Disconnect()

	service, err := manager.OpenService(appmeta.ServiceName)
	if err != nil {
		return fmt.Errorf("open service: %w", err)
	}
	defer service.Close()

	if err := fn(service); err != nil {
		return fmt.Errorf("manage service: %w", err)
	}
	return nil
}

func stop(service *mgr.Service) error {
	status, err := service.Query()
	if err != nil {
		return err
	}
	if status.State == svc.Stopped {
		return nil
	}
	if status.State != svc.StopPending {
		if _, err := service.Control(svc.Stop); err != nil && !errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
			return err
		}
	}
	return waitForState(service, svc.Stopped)
}

func waitForState(service *mgr.Service, desired svc.State) error {
	deadline := time.Now().Add(stateChangeTimeout)
	for time.Now().Before(deadline) {
		status, err := service.Query()
		if err != nil {
			return err
		}
		if status.State == desired {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for service state %d", desired)
}
