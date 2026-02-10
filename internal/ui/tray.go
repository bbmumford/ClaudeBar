package ui

import (
	"claudebar/internal/api"
	"claudebar/internal/assets"
	"fmt"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

// TrayManager handles the system tray icon and menu
type TrayManager struct {
	app           fyne.App
	menu          *fyne.Menu
	usageItems    []*fyne.MenuItem
	onShowOverlay func()
	onHideOverlay func()
	onSettings    func()
	onRefresh     func()
	onQuit        func()
	overlayShown  bool
}

// NewTrayManager creates a new tray manager
func NewTrayManager(app fyne.App) *TrayManager {
	return &TrayManager{
		app:          app,
		overlayShown: true,
	}
}

// SetCallbacks sets the callback functions for tray actions
func (t *TrayManager) SetCallbacks(
	onShowOverlay, onHideOverlay, onSettings, onRefresh, onQuit func(),
) {
	t.onShowOverlay = onShowOverlay
	t.onHideOverlay = onHideOverlay
	t.onSettings = onSettings
	t.onRefresh = onRefresh
	t.onQuit = onQuit
}

// Setup initializes the system tray
func (t *TrayManager) Setup() error {
	if desk, ok := t.app.(desktop.App); ok {
		// Create menu items
		toggleItem := fyne.NewMenuItem("Hide Overlay", t.toggleOverlay)

		// Usage display items (will be updated)
		t.usageItems = []*fyne.MenuItem{
			fyne.NewMenuItem("Session: --", nil),
			fyne.NewMenuItem("Weekly: --", nil),
		}
		// Disable clicking on info items
		for _, item := range t.usageItems {
			item.Disabled = true
		}

		separator := fyne.NewMenuItemSeparator()

		refreshItem := fyne.NewMenuItem("Refresh Now", func() {
			if t.onRefresh != nil {
				t.onRefresh()
			}
		})

		settingsItem := fyne.NewMenuItem("Settings...", func() {
			if t.onSettings != nil {
				t.onSettings()
			}
		})

		quitItem := fyne.NewMenuItem("Quit", func() {
			if t.onQuit != nil {
				t.onQuit()
			}
		})

		// Build menu
		t.menu = fyne.NewMenu("ClaudeBar",
			toggleItem,
			separator,
			t.usageItems[0],
			t.usageItems[1],
			separator,
			refreshItem,
			settingsItem,
			separator,
			quitItem,
		)

		desk.SetSystemTrayMenu(t.menu)
		desk.SetSystemTrayIcon(assets.TrayIcon())
		log.Println("System tray initialized")
		return nil
	}

	return fmt.Errorf("system tray not supported on this platform")
}

// toggleOverlay handles the show/hide toggle
func (t *TrayManager) toggleOverlay() {
	if t.overlayShown {
		if t.onHideOverlay != nil {
			t.onHideOverlay()
		}
		t.overlayShown = false
		// Update menu item text
		if t.menu != nil && len(t.menu.Items) > 0 {
			t.menu.Items[0].Label = "Show Overlay"
			t.menu.Refresh()
		}
	} else {
		if t.onShowOverlay != nil {
			t.onShowOverlay()
		}
		t.overlayShown = true
		if t.menu != nil && len(t.menu.Items) > 0 {
			t.menu.Items[0].Label = "Hide Overlay"
			t.menu.Refresh()
		}
	}
}

// UpdateUsage updates the usage display in the tray menu
func (t *TrayManager) UpdateUsage(data *api.UsageData) {
	if data == nil || t.usageItems == nil {
		return
	}

	// Update session usage with reset timer in brackets
	if t.usageItems[0] != nil {
		reset := api.TimeUntilReset(data.FiveHour.ResetsAt)
		t.usageItems[0].Label = fmt.Sprintf("Session: %.0f%% (resets %s)", data.FiveHour.Utilization, reset)
	}

	// Update weekly usage with reset timer in brackets
	if t.usageItems[1] != nil {
		reset := api.TimeUntilReset(data.SevenDay.ResetsAt)
		t.usageItems[1].Label = fmt.Sprintf("Weekly: %.0f%% (resets %s)", data.SevenDay.Utilization, reset)
	}

	// Refresh the menu
	if t.menu != nil {
		t.menu.Refresh()
	}
}

// SetOverlayState updates the tray to reflect overlay visibility
func (t *TrayManager) SetOverlayState(shown bool) {
	t.overlayShown = shown
	if t.menu != nil && len(t.menu.Items) > 0 {
		if shown {
			t.menu.Items[0].Label = "Hide Overlay"
		} else {
			t.menu.Items[0].Label = "Show Overlay"
		}
		t.menu.Refresh()
	}
}
