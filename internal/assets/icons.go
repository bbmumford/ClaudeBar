package assets

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed tray.png
var trayIconData []byte

//go:embed app.png
var appIconData []byte

// TrayIcon returns the system tray icon resource
func TrayIcon() fyne.Resource {
	return fyne.NewStaticResource("tray.png", trayIconData)
}

// AppIcon returns the application icon resource
func AppIcon() fyne.Resource {
	return fyne.NewStaticResource("app.png", appIconData)
}
