//go:build windows

package tray

import (
	"fmt"

	"github.com/lxn/walk"
	"github.com/phyatt/win-r-protector/internal/applog"
	"github.com/phyatt/win-r-protector/internal/appmeta"
	"github.com/phyatt/win-r-protector/internal/elevate"
	"github.com/phyatt/win-r-protector/internal/keyboardhook"
	"github.com/phyatt/win-r-protector/internal/servicecontrol"
	"github.com/phyatt/win-r-protector/internal/startup"
	"golang.org/x/sys/windows/svc"
)

type ui struct {
	window    *walk.MainWindow
	icon      *walk.NotifyIcon
	status    *walk.Action
	start     *walk.Action
	stop      *walk.Action
	restart   *walk.Action
	uninstall *walk.Action

	lastStatusError string
}

func Run(initialWarning string) error {
	window, err := walk.NewMainWindow()
	if err != nil {
		return err
	}
	defer window.Dispose()

	notifyIcon, err := walk.NewNotifyIcon(window)
	if err != nil {
		return err
	}
	defer notifyIcon.Dispose()

	trayIcon, err := walk.NewIconFromResourceId(1)
	if err != nil {
		return err
	}
	defer trayIcon.Dispose()

	if err := notifyIcon.SetIcon(trayIcon); err != nil {
		return err
	}
	if err := notifyIcon.SetToolTip(appmeta.Name); err != nil {
		return err
	}

	view := &ui{window: window, icon: notifyIcon}
	if err := view.buildMenu(); err != nil {
		return err
	}
	view.refresh()
	notifyIcon.MouseDown().Attach(func(_, _ int, _ walk.MouseButton) {
		view.refresh()
	})

	if err := notifyIcon.SetVisible(true); err != nil {
		return err
	}

	hook, err := keyboardhook.Start()
	if err != nil {
		applog.Errorf("start Win+R keyboard protection: %v", err)
		initialWarning = appendWarning(initialWarning, fmt.Sprintf("Win+R protection could not start:\n\n%v", err))
	} else {
		defer func() {
			if err := hook.Close(); err != nil {
				applog.Errorf("stop Win+R keyboard protection: %v", err)
			}
		}()
		stopWatching := make(chan struct{})
		defer close(stopWatching)
		go showWinRAttempts(view, hook.Attempts(), stopWatching)
	}

	if initialWarning != "" {
		walk.MsgBox(window, appmeta.Name, initialWarning, walk.MsgBoxOK|walk.MsgBoxIconWarning)
	}

	window.Run()
	return nil
}

func ShowError(message string) {
	walk.MsgBox(nil, appmeta.Name, message, walk.MsgBoxOK|walk.MsgBoxIconError)
}

func (view *ui) buildMenu() error {
	about := walk.NewAction()
	if err := about.SetText("About"); err != nil {
		return err
	}
	about.Triggered().Attach(func() {
		walk.MsgBox(view.window, appmeta.Name, "Win+R protection and Windows service management are running.", walk.MsgBoxOK|walk.MsgBoxIconInformation)
	})

	serviceMenu, err := walk.NewMenu()
	if err != nil {
		return err
	}
	manage := walk.NewMenuAction(serviceMenu)
	if err := manage.SetText("Manage Service"); err != nil {
		return err
	}

	view.status = walk.NewAction()
	view.status.SetEnabled(false)
	view.start = action("Start Service", func() { view.runElevated(elevate.Start, "Service started.") })
	view.stop = action("Stop Service", func() { view.runElevated(elevate.Stop, "Service stopped.") })
	view.restart = action("Restart Service", func() { view.runElevated(elevate.Restart, "Service restarted.") })
	view.uninstall = action("Uninstall Service", view.uninstallService)
	exit := action("Exit Tray", func() { walk.App().Exit(0) })

	serviceActions := serviceMenu.Actions()
	for _, item := range []*walk.Action{
		view.status,
		walk.NewSeparatorAction(),
		view.start,
		view.stop,
		view.restart,
		view.uninstall,
		walk.NewSeparatorAction(),
		exit,
	} {
		if err := serviceActions.Add(item); err != nil {
			return err
		}
	}

	root := view.icon.ContextMenu().Actions()
	if err := root.Add(about); err != nil {
		return err
	}
	if err := root.Add(walk.NewSeparatorAction()); err != nil {
		return err
	}
	return root.Add(manage)
}

func showWinRAttempts(view *ui, attempts <-chan struct{}, stop <-chan struct{}) {
	for {
		select {
		case <-attempts:
			view.window.Synchronize(func() {
				walk.MsgBox(
					view.window,
					appmeta.Name,
					"That shortcut key is blocked.  It is commonly used by scammers.\n\nPlease hang up immediately and call a trusted family member!",
					walk.MsgBoxOK|walk.MsgBoxIconWarning,
				)
			})
		case <-stop:
			return
		}
	}
}

func appendWarning(existing, warning string) string {
	if existing == "" {
		return warning
	}
	return existing + "\n\n" + warning
}

func action(text string, handler func()) *walk.Action {
	item := walk.NewAction()
	item.SetText(text)
	item.Triggered().Attach(handler)
	return item
}

func (view *ui) refresh() {
	info, err := servicecontrol.Status()
	if err != nil {
		if message := err.Error(); message != view.lastStatusError {
			applog.Errorf("query service status from tray: %v", err)
			view.lastStatusError = message
		}
		view.status.SetText("Status: unavailable")
		view.start.SetEnabled(false)
		view.stop.SetEnabled(false)
		view.restart.SetEnabled(false)
		view.uninstall.SetEnabled(false)
		return
	}
	view.lastStatusError = ""

	view.status.SetText("Status: " + servicecontrol.StateText(info))
	view.start.SetEnabled(info.Installed && info.State == svc.Stopped)
	view.stop.SetEnabled(info.Installed && (info.State == svc.Running || info.State == svc.Paused))
	view.restart.SetEnabled(info.Installed && info.State == svc.Running)
	view.uninstall.SetEnabled(info.Installed)
}

func (view *ui) runElevated(action elevate.Action, success string) bool {
	exitCode, err := elevate.Run(action)
	if err != nil {
		applog.Errorf("run administrative action %q: %v", action, err)
		walk.MsgBox(view.window, appmeta.Name, fmt.Sprintf("The administrative action failed:\n\n%v", err), walk.MsgBoxOK|walk.MsgBoxIconError)
		view.refresh()
		return false
	}
	if exitCode != 0 {
		applog.Errorf("administrative action %q exited with code %d", action, exitCode)
		walk.MsgBox(view.window, appmeta.Name, "The administrative action did not complete successfully.", walk.MsgBoxOK|walk.MsgBoxIconError)
		view.refresh()
		return false
	}
	walk.MsgBox(view.window, appmeta.Name, success, walk.MsgBoxOK|walk.MsgBoxIconInformation)
	view.refresh()
	return true
}

func (view *ui) uninstallService() {
	if !view.runElevated(elevate.Uninstall, "Service uninstalled. The application file was left in place.") {
		return
	}
	if err := startup.Unregister(); err != nil {
		applog.Errorf("remove tray logon startup after service uninstall: %v", err)
		walk.MsgBox(view.window, appmeta.Name, fmt.Sprintf("The service was removed, but logon startup could not be removed:\n\n%v", err), walk.MsgBoxOK|walk.MsgBoxIconWarning)
	}
}
