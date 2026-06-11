package main

import (
	"embed"
	"log"
	"runtime"

	"gittar/internal/service"
	"gittar/internal/tray"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

// Wails uses Go's `embed` package to embed the frontend files into the binary.
//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// 1. Initialize services
	appService := service.NewAppService()
	notifier := notifications.New()

	// 2. Create the Wails application
	opts := application.Options{
		Name:        "Gittar",
		Description: "GitLab Enterprise Control Panel",
		Services: []application.Service{
			application.NewService(appService),
			application.NewService(notifier),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	}
	app := application.New(opts)

	// 3. Configure the main window (Dynamic cross-platform options)
	windowOptions := application.WebviewWindowOptions{
		Title:  "Gittar",
		Width:  1200,
		Height: 800,
		BackgroundColour: application.NewRGBA(13, 17, 23, 150), // Transparent dark tint for vibrant Acrylic blur
		URL:              "/",
	}

	switch runtime.GOOS {
	case "darwin":
		windowOptions.Mac = application.MacWindow{
			InvisibleTitleBarHeight: 40,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		}
	case "windows":
		windowOptions.Frameless = true
		windowOptions.Windows = application.WindowsWindow{
			BackdropType:                      application.Acrylic, // Sexy Windows Acrylic blur
			Theme:                             application.Dark,
			DisableFramelessWindowDecorations: false, // Maintain rounded corners & drop shadow on Windows 11
		}
	}

	win := app.Window.NewWithOptions(windowOptions)

	// 4. Initialize system tray service
	ts := tray.NewTrayService(app, win, notifier)
	appService.SetTray(ts)

	// 5. Run the application
	err := app.Run()
	if err != nil {
		log.Fatal(err)
	}
}
