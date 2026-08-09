//go:build windows

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/phyatt/win-r-protector/internal/applog"
	"github.com/phyatt/win-r-protector/internal/elevate"
	"github.com/phyatt/win-r-protector/internal/servicecontrol"
	"github.com/phyatt/win-r-protector/internal/singleinstance"
	"github.com/phyatt/win-r-protector/internal/startup"
	"github.com/phyatt/win-r-protector/internal/tray"
	"golang.org/x/sys/windows/svc"
)

const internalActionPrefix = "--internal-action="

func main() {
	_, logErr := applog.Start()
	if logErr == nil {
		defer applog.Close()
	}

	isService, err := servicecontrol.IsServiceProcess()
	if err != nil {
		applog.Errorf("detect process mode: %v", err)
		return
	}
	if isService {
		if err := servicecontrol.Run(); err != nil {
			applog.Errorf("run Windows service: %v", err)
		}
		return
	}

	if action, ok := internalAction(); ok {
		if err := performInternalAction(action); err != nil {
			applog.Errorf("internal administrative action %q: %v", action, err)
			os.Exit(1)
		}
		return
	}

	lock, acquired, err := singleinstance.Acquire()
	if err != nil {
		applog.Errorf("acquire tray single-instance lock: %v", err)
		tray.ShowError(err.Error())
		return
	}
	if !acquired {
		return
	}
	defer lock.Close()

	warning := ""
	if logErr != nil {
		warning = fmt.Sprintf("Diagnostic logging could not be started:\n\n%v", logErr)
	}
	warning = appendWarning(warning, prepareInteractiveMode())
	if err := tray.Run(warning); err != nil {
		applog.Errorf("run tray application: %v", err)
		tray.ShowError(fmt.Sprintf("Unable to start the tray application:\n\n%v", err))
	}
}

func internalAction() (elevate.Action, bool) {
	if len(os.Args) != 2 || !strings.HasPrefix(os.Args[1], internalActionPrefix) {
		return "", false
	}
	action := elevate.Action(strings.TrimPrefix(os.Args[1], internalActionPrefix))
	return action, true
}

func performInternalAction(action elevate.Action) error {
	switch action {
	case elevate.Install:
		return servicecontrol.Install()
	case elevate.Start:
		return servicecontrol.Start()
	case elevate.Stop:
		return servicecontrol.Stop()
	case elevate.Restart:
		return servicecontrol.Restart()
	case elevate.Uninstall:
		return servicecontrol.Uninstall()
	default:
		return fmt.Errorf("unknown internal action %q", action)
	}
}

func prepareInteractiveMode() string {
	info, err := servicecontrol.Status()
	if err != nil {
		return launchFailure("Could not inspect the Windows service", err)
	}
	if !info.Installed {
		exitCode, err := elevate.Run(elevate.Install)
		if err != nil {
			return launchFailure("The Windows service could not be installed", err)
		}
		if exitCode != 0 {
			return launchFailure("The Windows service installation did not complete successfully", fmt.Errorf("elevated helper exited with code %d", exitCode))
		}
	} else if info.State == svc.StartPending || info.State == svc.ContinuePending {
		if err := servicecontrol.WaitForRunning(); err != nil {
			return launchFailure("The Windows service did not finish starting", err)
		}
	} else if info.State != svc.Running {
		exitCode, err := elevate.Run(elevate.Start)
		if err != nil {
			return launchFailure("The Windows service could not be started", err)
		}
		if exitCode != 0 {
			return launchFailure("The Windows service did not start successfully", fmt.Errorf("elevated helper exited with code %d", exitCode))
		}
	}

	info, err = servicecontrol.Status()
	if err != nil {
		return launchFailure("Could not verify the Windows service after startup", err)
	}
	if !info.Installed || info.State != svc.Running {
		return launchFailure("The Windows service is not running", fmt.Errorf("current state is %s", servicecontrol.StateText(info)))
	}

	if err := startup.Register(); err != nil {
		return launchFailure("The service is running, but tray startup at logon could not be registered", err)
	}
	return ""
}

func launchFailure(message string, err error) string {
	applog.Errorf("%s: %v", message, err)
	result := fmt.Sprintf("%s:\n\n%v", message, err)
	if path := applog.Path(); path != "" {
		result += "\n\nLog: " + path
	}
	return result
}

func appendWarning(existing, warning string) string {
	if warning == "" {
		return existing
	}
	if existing == "" {
		return warning
	}
	return existing + "\n\n" + warning
}
