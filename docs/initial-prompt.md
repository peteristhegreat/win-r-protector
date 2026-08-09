Set up the Go dependencies and minimal project structure for a **Windows-only utility** with these capabilities:

* Runs as a Windows Service using `golang.org/x/sys/windows/svc`
* Manages its service registration/state using `golang.org/x/sys/windows/svc/mgr`
* Uses `github.com/lxn/walk` for:

  * system tray icon
  * tray context menu
  * native popup/message dialogs
  * potential future settings/status window
* Uses `golang.org/x/sys/windows` for direct Win32 access where needed
* Avoid CGO and avoid Fyne/cross-platform GUI frameworks
* The application should require **no command-line interface for normal use**

Implement the following startup behavior:

1. When the EXE starts, determine whether Windows launched it through the Service Control Manager.

   * If so, run in Windows Service mode and do not create any desktop UI.

2. If launched interactively by the user:

   * Check whether the application's Windows service is installed.
   * If it is not installed:

     * Relaunch/self-elevate using the Windows UAC `runas` mechanism.
     * The elevated instance should install the current executable as the Windows service.
     * Configure the service appropriately for automatic startup.
     * Start the service after installation.
     * Return to normal interactive/tray operation.
   * Avoid displaying a console window.

3. If the service is already installed:

   * Ensure only one interactive tray instance exists for the current user/session.
   * Start the tray UI.
   * Do not start another copy of the service simply because the EXE was manually launched.

4. The tray context menu should contain normal application actions plus a **Manage Service** section containing:

   * Service status
   * Start Service
   * Stop Service
   * Restart Service
   * Uninstall Service
   * Exit Tray

5. Enable/disable Start/Stop actions based on the current SCM service state.

6. Administrative tray actions should elevate only when necessary.

   * The normal tray process should run unelevated.
   * Start/stop/uninstall/install operations requiring administrator privileges should self-elevate through UAC.
   * Do not keep the tray application permanently elevated.

7. Uninstall Service should:

   * stop the service if necessary
   * remove it from the SCM
   * leave the currently running EXE/file in place
   * report success/failure through a Walk popup

8. Keep the service and tray UI as separate process modes even though they are implemented by the same EXE.

   * Never attempt to display tray UI or dialogs from the service/session 0.
   * Structure the code so IPC between the service and tray process can be added later, preferably using Windows named pipes.

8. Use a **single EXE with two possible runtime roles**, implemented as separate process instances:

   * When launched by the Windows Service Control Manager, the EXE runs in **service mode** in Session 0.
   * When launched interactively in a logged-in user's session, the same EXE runs in **tray/UI mode**.
   * Never attempt to create tray icons, windows, or dialogs from the SCM/service instance.
   * Keep the two roles cleanly separated internally even though they share one executable and common packages.
   * Design the architecture so the service and tray instances can communicate later through Windows named pipes or another appropriate local IPC mechanism.

9. Ensure the tray/UI instance starts automatically when a user logs into Windows:

   * Install an appropriate per-user logon startup mechanism for the tray instance.
   * Prefer a simple Windows-native mechanism such as `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` unless there is a strong reason to use a Scheduled Task.
   * The logon-started instance should detect that the service is already installed/running and simply enter tray mode.
   * Ensure only one tray instance runs per interactive user session.
   * Removing/uninstalling the application should also remove its logon-startup registration.
   * Do not have the Windows service attempt to launch the tray application into the user's session.


9.5 Add minimal compilable scaffolding for:

   * SCM/service-mode detection
   * Windows service handler
   * service install/start/stop/restart/uninstall helpers
   * UAC self-elevation helper
   * single-instance tray detection
   * Walk tray icon and context menu
   * simple Walk popup/message box
   * clean application shutdown

10. Use internal/private command-line arguments only as an implementation detail for elevated relaunches if necessary (for example an internal install/uninstall action). These should not be presented as a user-facing CLI.

11. Add Windows build constraints where appropriate.

12. Keep the implementation small, idiomatic, and modular. Do not add application/business logic yet.

13. Install the appropriate dependencies with `go get`, run `go mod tidy`, and verify the project builds successfully for Windows.

Prefer standard Go and `golang.org/x/sys/windows` APIs over additional dependencies when practical.
