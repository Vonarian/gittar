package tray

import (
	_ "embed"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

//go:embed tray_icon.png
var trayIcon []byte

// TrayService manages the system tray icon, labels, window visibility, and desktop notifications.
type TrayService struct {
	app      *application.App
	window   application.Window
	systray  *application.SystemTray
	notifier *notifications.NotificationService
}

// NewTrayService initializes and registers the tray menu.
func NewTrayService(app *application.App, window application.Window, notifier *notifications.NotificationService) *TrayService {
	ts := &TrayService{
		app:      app,
		window:   window,
		notifier: notifier,
	}
	ts.init()
	return ts
}

func (ts *TrayService) init() {
	ts.systray = ts.app.SystemTray.New()
	if runtime.GOOS == "windows" {
		ts.systray.SetIcon(trayIcon)
	}
	ts.systray.SetLabel("Gittar")
	ts.systray.SetTooltip("GitLab Enterprise Control Panel")

	// Set click handler to focus the main window
	ts.systray.OnClick(func() {
		ts.window.Show()
		ts.window.Focus()
	})
}

// RequestNotificationAuth requests authorization for desktop notifications on macOS after the app event loop starts.
func (ts *TrayService) RequestNotificationAuth() {
	if ts.notifier == nil || runtime.GOOS != "darwin" {
		return
	}

	execPath, err := os.Executable()
	if err != nil || !strings.Contains(execPath, ".app/Contents/MacOS") {
		// Skip requesting notification permissions if not running inside a proper macOS app bundle
		// to avoid triggering NSException crashes in raw CLI binaries.
		return
	}

	go func() {
		_, _ = ts.notifier.RequestNotificationAuthorization()
	}()
}

// UpdateTray adjusts the system tray ticker text according to the pipeline statuses.
func (ts *TrayService) UpdateTray(passing, failing, running int) {
	var label string
	if failing > 0 {
		label = fmt.Sprintf("Gittar (✗ %d)", failing)
	} else if running > 0 {
		label = fmt.Sprintf("Gittar (⟳ %d)", running)
	} else if passing > 0 {
		label = "Gittar (✓)"
	} else {
		label = "Gittar"
	}
	ts.systray.SetLabel(label)
}

// Notify triggers a native operating system notification.
func (ts *TrayService) Notify(title, body string) error {
	if ts.notifier == nil {
		fmt.Println("[Go Backend] Notify failed: ts.notifier is nil")
		return fmt.Errorf("notifier not initialized")
	}
	err := ts.notifier.SendNotification(notifications.NotificationOptions{
		ID:    fmt.Sprintf("gittar-%d", time.Now().UnixNano()),
		Title: title,
		Body:  body,
	})
	if err != nil {
		fmt.Printf("[Go Backend] SendNotification error: %v\n", err)
		return err
	}
	fmt.Printf("[Go Backend] SendNotification succeeded for title: %q, body: %q\n", title, body)
	return nil
}
