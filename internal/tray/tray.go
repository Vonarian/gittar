package tray

import (
	"fmt"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

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
	ts.systray.SetLabel("Gittar")
	ts.systray.SetTooltip("GitLab Enterprise Control Panel")

	// Set click handler to focus the main window
	ts.systray.OnClick(func() {
		ts.window.Show()
		ts.window.Focus()
	})

	// Request macOS authorization for notifications on start
	if ts.notifier != nil {
		go func() {
			_, _ = ts.notifier.RequestNotificationAuthorization()
		}()
	}
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
func (ts *TrayService) Notify(title, body string) {
	if ts.notifier == nil {
		return
	}
	_ = ts.notifier.SendNotification(notifications.NotificationOptions{
		Title: title,
		Body:  body,
	})
}
