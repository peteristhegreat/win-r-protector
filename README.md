# Win-R Protector

A Windows-only background service and per-user tray application that intercepts Win+R.

## Build

Go 1.26 or newer is expected. From PowerShell:

```powershell
.\build.ps1
```

The Windows GUI executable is written to `dist\win-r-protector.exe`; it does not open a console window.
The build embeds `icons\win-r-protect.ico` as both the executable and tray icon. The pinned resource generator is downloaded automatically by Go when needed.

## Install and run

Double-click `win-r-protector.exe`. On first launch it asks for UAC approval, installs and starts its automatic Windows service, registers the tray for the current user's next logon, and then remains running as an unelevated tray process.

While the tray process is running, a native low-level keyboard hook suppresses Win+R and displays an application-owned warning: **Win-R attempt detected**. Other Windows-key shortcuts pass through unchanged.

Right-click the shield tray icon and open **Manage Service** to view status, start, stop, restart, uninstall the service, or exit the tray. Administrative actions request UAC approval only when selected. Uninstalling removes the service and the current user's logon registration but deliberately leaves the executable in place.

The service never creates desktop UI. `internal/ipc` is reserved for a future secured named-pipe connection between the Session 0 service and the per-user tray process.
