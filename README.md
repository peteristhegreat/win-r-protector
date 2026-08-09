# Win-R Protector

Minimal Windows-only scaffolding for a background service and per-user tray application in one executable.

## Build

Go 1.26 or newer is expected. From PowerShell:

```powershell
.\build.ps1
```

The Windows GUI executable is written to `dist\win-r-protector.exe`; it does not open a console window.
The build embeds `icons\win-r-protect.ico` as both the executable and tray icon. The pinned resource generator is downloaded automatically by Go when needed.

## Install and run

Double-click `win-r-protector.exe`. On first launch it asks for UAC approval, installs and starts its automatic Windows service, registers the tray for the current user's next logon, and then remains running as an unelevated tray process.

Right-click the shield tray icon and open **Manage Service** to view status, start, stop, restart, uninstall the service, or exit the tray. Administrative actions request UAC approval only when selected. Uninstalling removes the service and the current user's logon registration but deliberately leaves the executable in place.

The service never creates desktop UI. `internal/ipc` is reserved for a future secured named-pipe connection between the Session 0 service and the per-user tray process.
