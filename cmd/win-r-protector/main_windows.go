//go:build windows

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/phyatt/win-r-protector/internal/elevate"
	"github.com/phyatt/win-r-protector/internal/servicecontrol"
	"github.com/phyatt/win-r-protector/internal/singleinstance"
	"github.com/phyatt/win-r-protector/internal/startup"
	"github.com/phyatt/win-r-protector/internal/tray"
)

const internalActionPrefix = "--internal-action="

func main() {
	isService, err := servicecontrol.IsServiceProcess()
	if err != nil {
		return
	}
	if isService {
		_ = servicecontrol.Run()
		return
	}

	if action, ok := internalAction(); ok {
		if err := performInternalAction(action); err != nil {
			os.Exit(1)
		}
		return
	}

	lock, acquired, err := singleinstance.Acquire()
	if err != nil {
		tray.ShowError(err.Error())
		return
	}
	if !acquired {
		return
	}
	defer lock.Close()

	warning := prepareInteractiveMode()
	if err := tray.Run(warning); err != nil {
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
		return fmt.Sprintf("Could not inspect the Windows service:\n\n%v", err)
	}
	if !info.Installed {
		exitCode, err := elevate.Run(elevate.Install)
		if err != nil {
			return fmt.Sprintf("The Windows service could not be installed:\n\n%v", err)
		}
		if exitCode != 0 {
			return "The Windows service installation did not complete successfully."
		}
	}
	if err := startup.Register(); err != nil {
		return fmt.Sprintf("The service is available, but tray startup at logon could not be registered:\n\n%v", err)
	}
	return ""
}
