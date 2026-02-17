package app

import (
	"fmt"
	"log"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/theme"

	"claudebar/internal/api"
	"claudebar/internal/assets"
	"claudebar/internal/config"
	"claudebar/internal/hotkeys"
	"claudebar/internal/platform"
	"claudebar/internal/ui"
)

// App is the main application
type App struct {
	fyneApp     fyne.App
	config      *config.Config
	apiClient   *api.Client
	authManager *api.AuthManager
	hotkeyMgr   *hotkeys.Manager

	// UI components
	tray     *ui.TrayManager
	overlay  *ui.OverlayWindow
	settings *ui.SettingsDialog

	// State
	mu                sync.RWMutex
	running           bool
	refreshTimer      *time.Ticker
	stopChan          chan struct{}
	consecutiveErrors    int
	rateLimitBackoff     time.Duration
	lastSessionThreshold float64 // last threshold that triggered a session notification
	lastWeeklyThreshold  float64 // last threshold that triggered a weekly notification
}

// Run starts the application
func Run() error {
	a := &App{
		stopChan: make(chan struct{}),
	}

	// Initialize Fyne app
	a.fyneApp = app.NewWithID("com.claudebar.app")
	a.fyneApp.Settings().SetTheme(theme.DarkTheme())
	a.fyneApp.SetIcon(assets.AppIcon())

	// Load config
	a.config = config.Get()

	// Initialize API client
	a.apiClient = api.NewClient()
	a.authManager = api.NewAuthManager(a.apiClient)

	// Initialize hotkey manager
	a.hotkeyMgr = hotkeys.NewManager()

	// Initialize UI
	if err := a.initUI(); err != nil {
		return err
	}

	// Authenticate
	go a.authenticate()

	// Start hotkey listener
	a.hotkeyMgr.SetSnapCallback(a.handleSnapHotkey)
	a.hotkeyMgr.SetToggleCallback(a.handleToggleHotkey)
	if err := a.hotkeyMgr.Start(); err != nil {
		log.Printf("Warning: Failed to start hotkey listener: %v", err)
	}

	// Start refresh loop
	go a.refreshLoop()

	a.running = true

	// Run the app (blocking)
	a.fyneApp.Run()

	// Cleanup
	a.shutdown()

	return nil
}

// initUI initializes all UI components
func (a *App) initUI() error {
	// Create overlay window
	a.overlay = ui.NewOverlayWindow(a.fyneApp)
	if err := a.overlay.Setup(); err != nil {
		return err
	}

	// Create tray manager
	a.tray = ui.NewTrayManager(a.fyneApp)
	a.tray.SetCallbacks(
		a.showOverlay,
		a.hideOverlay,
		a.showSettings,
		a.refreshNow,
		a.quit,
	)
	if err := a.tray.Setup(); err != nil {
		log.Printf("Warning: System tray setup failed: %v", err)
	}

	// Show overlay if enabled
	if a.config.OverlayEnabled {
		a.overlay.Show()
	}

	return nil
}

// authenticate attempts to authenticate with Claude
func (a *App) authenticate() {
	log.Println("Attempting authentication...")

	fyne.Do(func() {
		a.overlay.SetStatus("Authenticating...")
	})

	if err := a.authManager.Initialize(); err != nil {
		log.Printf("Authentication failed: %v", err)
		log.Println("Please set session key in Settings")
		fyne.Do(func() {
			a.overlay.SetStatus("Set session key in Settings")
		})
		return
	}

	log.Println("Authentication successful")

	fyne.Do(func() {
		a.overlay.SetStatus("Fetching usage...")
	})

	// Fetch initial usage
	a.fetchUsage()
}

// refreshLoop periodically fetches usage data with idle detection
func (a *App) refreshLoop() {
	const idleThreshold = 300 // 5 minutes in seconds
	const idleInterval = 300  // poll every 5 min when idle

	normalInterval := time.Duration(a.config.RefreshInterval) * time.Second
	if normalInterval < 15*time.Second {
		normalInterval = 15 * time.Second
	}

	a.refreshTimer = time.NewTicker(normalInterval)
	defer a.refreshTimer.Stop()

	wasIdle := false

	for {
		select {
		case <-a.refreshTimer.C:
			idleSec := platform.Features.GetIdleSeconds()

			if idleSec > idleThreshold {
				// User is idle — slow down
				if !wasIdle {
					wasIdle = true
					a.refreshTimer.Reset(time.Duration(idleInterval) * time.Second)
					log.Printf("User idle (%ds), reducing refresh rate", idleSec)
				}
				a.fetchUsage()
			} else {
				// User is active
				if wasIdle {
					wasIdle = false
					a.refreshTimer.Reset(normalInterval)
					log.Println("User active, restoring normal refresh rate")
					a.fetchUsage() // immediate refresh on wake
				} else {
					a.fetchUsage()
				}
			}
		case <-a.stopChan:
			return
		}
	}
}

// fetchUsage retrieves and updates usage data
func (a *App) fetchUsage() {
	if !a.authManager.IsAuthenticated() {
		return
	}

	// If we're in rate-limit backoff, skip this tick
	a.mu.RLock()
	backoff := a.rateLimitBackoff
	a.mu.RUnlock()
	if backoff > 0 {
		a.mu.Lock()
		a.rateLimitBackoff = 0
		a.mu.Unlock()
		log.Printf("Rate limit backoff: waiting %v before next fetch", backoff)
		time.Sleep(backoff)
	}

	usage, err := a.apiClient.FetchUsage()
	if err != nil {
		log.Printf("Failed to fetch usage: %v", err)

		a.mu.Lock()
		a.consecutiveErrors++
		errCount := a.consecutiveErrors
		a.mu.Unlock()

		switch {
		case err == api.ErrSessionExpired:
			log.Println("Session key expired, attempting browser refresh...")
			fyne.Do(func() {
				a.overlay.SetStatus("Session expired - refreshing...")
			})
			if refreshErr := a.authManager.RefreshFromBrowser(); refreshErr != nil {
				log.Printf("Re-authentication failed: %v", refreshErr)
				fyne.Do(func() {
					a.overlay.SetStatus("Session expired - update key in Settings")
				})
			}

		case err == api.ErrUnauthorized:
			log.Println("Unauthorized, attempting browser refresh...")
			fyne.Do(func() {
				a.overlay.SetStatus("Auth failed - refreshing...")
			})
			if refreshErr := a.authManager.RefreshFromBrowser(); refreshErr != nil {
				log.Printf("Re-authentication failed: %v", refreshErr)
				fyne.Do(func() {
					a.overlay.SetStatus("Auth failed - update key in Settings")
				})
			}

		case err == api.ErrRateLimited:
			// Exponential backoff: 30s, 60s, 120s, capped at 5 min
			backoffSec := 30 * (1 << min(errCount-1, 3))
			a.mu.Lock()
			a.rateLimitBackoff = time.Duration(backoffSec) * time.Second
			a.mu.Unlock()
			log.Printf("Rate limited, backing off %ds", backoffSec)
			fyne.Do(func() {
				a.overlay.SetStatus(fmt.Sprintf("Rate limited - retry in %ds", backoffSec))
			})

		case err == api.ErrAPIUnavailable:
			fyne.Do(func() {
				a.overlay.SetStatus("Claude API unavailable")
			})

		default:
			// Transient error — show status only after multiple consecutive failures
			if errCount >= 3 {
				fyne.Do(func() {
					a.overlay.SetStatus("Connection error - retrying...")
				})
			}
		}
		return
	}

	// Success — reset error counters
	a.mu.Lock()
	a.consecutiveErrors = 0
	a.rateLimitBackoff = 0
	a.mu.Unlock()

	log.Printf("Usage fetched: 5h=%.0f%%, weekly=%.0f%%",
		usage.FiveHour.Utilization,
		usage.SevenDay.Utilization,
	)

	// Update UI on the Fyne main thread
	fyne.Do(func() {
		a.overlay.UpdateUsage(usage)
		a.tray.UpdateUsage(usage)
	})

	// Check notification thresholds
	a.checkAndNotify(usage)
}

// checkAndNotify sends OS notifications when usage crosses configured thresholds.
// Only notifies once per threshold crossing; resets when usage drops below.
func (a *App) checkAndNotify(usage *api.UsageData) {
	if !a.config.NotificationsEnabled || len(a.config.AlertThresholds) == 0 {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Check session (5-hour) usage
	sessionCrossed := highestCrossedThreshold(usage.FiveHour.Utilization, a.config.AlertThresholds)
	if sessionCrossed > a.lastSessionThreshold {
		a.lastSessionThreshold = sessionCrossed
		notif := fyne.NewNotification(
			"ClaudeBar: High Session Usage",
			fmt.Sprintf("Session usage at %.0f%% (threshold: %.0f%%)", usage.FiveHour.Utilization, sessionCrossed),
		)
		a.fyneApp.SendNotification(notif)
		log.Printf("Notification: session usage %.0f%% crossed %.0f%% threshold", usage.FiveHour.Utilization, sessionCrossed)
	} else if sessionCrossed < a.lastSessionThreshold {
		// Usage dropped (e.g., after reset) — allow re-notification
		a.lastSessionThreshold = sessionCrossed
	}

	// Check weekly (7-day) usage
	weeklyCrossed := highestCrossedThreshold(usage.SevenDay.Utilization, a.config.AlertThresholds)
	if weeklyCrossed > a.lastWeeklyThreshold {
		a.lastWeeklyThreshold = weeklyCrossed
		notif := fyne.NewNotification(
			"ClaudeBar: High Weekly Usage",
			fmt.Sprintf("Weekly usage at %.0f%% (threshold: %.0f%%)", usage.SevenDay.Utilization, weeklyCrossed),
		)
		a.fyneApp.SendNotification(notif)
		log.Printf("Notification: weekly usage %.0f%% crossed %.0f%% threshold", usage.SevenDay.Utilization, weeklyCrossed)
	} else if weeklyCrossed < a.lastWeeklyThreshold {
		a.lastWeeklyThreshold = weeklyCrossed
	}
}

// highestCrossedThreshold returns the highest threshold value that utilization
// meets or exceeds. Returns 0 if no threshold is crossed.
func highestCrossedThreshold(utilization float64, thresholds []float64) float64 {
	var highest float64
	for _, t := range thresholds {
		if utilization >= t && t > highest {
			highest = t
		}
	}
	return highest
}

// handleSnapHotkey handles snap hotkey events.
// Called from the hotkey goroutine, so must dispatch UI work to the Fyne thread.
func (a *App) handleSnapHotkey(pos platform.SnapPosition) {
	log.Printf("Snap hotkey: %s", pos)
	fyne.Do(func() {
		a.overlay.SnapTo(pos)

		// Show overlay if hidden
		if !a.overlay.IsVisible() {
			a.showOverlay()
		}
	})
}

// handleToggleHotkey handles Ctrl+Alt+. — opens the settings/tray app UI for quick access
func (a *App) handleToggleHotkey() {
	log.Println("Toggle hotkey: opening settings UI")
	fyne.Do(func() {
		a.showSettings()
	})
}

// showOverlay shows the overlay window
func (a *App) showOverlay() {
	a.overlay.Show()
	a.config.OverlayEnabled = true
	a.config.Save()
	a.tray.SetOverlayState(true)
}

// hideOverlay hides the overlay window
func (a *App) hideOverlay() {
	a.overlay.Hide()
	a.config.OverlayEnabled = false
	a.config.Save()
	a.tray.SetOverlayState(false)
}

// showSettings shows the settings dialog
func (a *App) showSettings() {
	if a.settings == nil {
		a.settings = ui.NewSettingsDialog(a.fyneApp, a.overlay.GetWindow())
		a.settings.SetCallbacks(
			a.authManager.SetManualSessionKey,
			a.authManager.RefreshFromBrowser,
			func() {
				// Refresh UI after settings save
				a.fetchUsage()
				// Update refresh interval
				if a.refreshTimer != nil {
					a.refreshTimer.Reset(time.Duration(a.config.RefreshInterval) * time.Second)
				}
				// Update opacity
				a.overlay.SetOpacity(a.config.OverlayOpacity)
			},
		)
	}
	a.settings.Show()
}

// refreshNow triggers an immediate usage refresh
func (a *App) refreshNow() {
	go a.fetchUsage()
}

// quit shuts down the application
func (a *App) quit() {
	a.shutdown()
	a.fyneApp.Quit()
}

// shutdown cleans up resources
func (a *App) shutdown() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return
	}
	a.running = false

	log.Println("Shutting down...")

	// Stop refresh loop
	close(a.stopChan)

	// Stop hotkey listener
	a.hotkeyMgr.Stop()

	log.Println("Shutdown complete")
}
