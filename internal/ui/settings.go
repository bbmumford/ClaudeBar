package ui

import (
	"errors"
	"fmt"

	"fyne.io/fyne/v2"

	"claudebar/internal/config"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// SettingsDialog manages the settings window
type SettingsDialog struct {
	app              fyne.App
	parent           fyne.Window
	config           *config.Config
	onSessionKeySet  func(string) error
	onRefreshBrowser func() error
	onSave           func()
}

// NewSettingsDialog creates a new settings dialog
func NewSettingsDialog(app fyne.App, parent fyne.Window) *SettingsDialog {
	return &SettingsDialog{
		app:    app,
		parent: parent,
		config: config.Get(),
	}
}

// SetCallbacks sets the callback functions
func (s *SettingsDialog) SetCallbacks(
	onSessionKeySet func(string) error,
	onRefreshBrowser func() error,
	onSave func(),
) {
	s.onSessionKeySet = onSessionKeySet
	s.onRefreshBrowser = onRefreshBrowser
	s.onSave = onSave
}

// Show displays the settings dialog
func (s *SettingsDialog) Show() {
	window := s.app.NewWindow("ClaudeBar Settings")
	window.Resize(fyne.NewSize(380, 430))

	// --- Authentication ---
	authLabel := widget.NewLabel("Authentication")
	authLabel.TextStyle = fyne.TextStyle{Bold: true}

	authStatus := widget.NewLabel("")
	if s.config.SessionKey != "" {
		keyPreview := s.config.SessionKey[:min(16, len(s.config.SessionKey))]
		authStatus.SetText(fmt.Sprintf("Active (%s...)", keyPreview))
	} else {
		authStatus.SetText("Not connected")
	}
	authStatus.Wrapping = fyne.TextWrapOff

	sessionKeyEntry := widget.NewPasswordEntry()
	sessionKeyEntry.SetPlaceHolder("sk-ant-sid01-...")

	setKeyBtn := widget.NewButton("Set Key", nil)
	setKeyBtn.OnTapped = func() {
		key := sessionKeyEntry.Text
		if key == "" || len(key) < 10 {
			dialog.ShowError(
				errors.New("invalid session key"),
				window,
			)
			return
		}
		if s.onSessionKeySet != nil {
			// Show loading state
			setKeyBtn.Disable()
			authStatus.SetText("Authenticating...")
			sessionKeyEntry.Disable()

			go func() {
				err := s.onSessionKeySet(key)
				fyne.Do(func() {
					setKeyBtn.Enable()
					sessionKeyEntry.Enable()
					if err != nil {
						authStatus.SetText("Failed")
						dialog.ShowError(err, window)
					} else {
						authStatus.SetText("Active")
						dialog.ShowInformation("Success", "Session key updated", window)
					}
				})
			}()
		}
	}

	helpBtn := widget.NewButton("Help", func() {
		dialog.ShowInformation("Session Key",
			"1. Open claude.ai and log in\n"+
				"2. Press F12 (DevTools)\n"+
				"3. Application > Cookies > claude.ai\n"+
				"4. Copy 'sessionKey' value\n"+
				"5. Paste above and click Set Key\n\n"+
				"Key expires periodically.\n"+
				"Chrome 127+ requires manual paste.",
			window,
		)
	})

	authSection := container.NewVBox(
		authLabel,
		authStatus,
		sessionKeyEntry,
		container.NewHBox(setKeyBtn, helpBtn),
	)

	// --- Display ---
	displayLabel := widget.NewLabel("Display")
	displayLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Opacity slider with live value label
	opacityBinding := binding.NewFloat()
	opacityBinding.Set(s.config.OverlayOpacity)
	opacitySlider := widget.NewSliderWithData(0.1, 1.0, opacityBinding)
	opacitySlider.Step = 0.05
	opacityValueLabel := widget.NewLabel(fmt.Sprintf("%.0f%%", s.config.OverlayOpacity*100))
	opacityBinding.AddListener(binding.NewDataListener(func() {
		v, _ := opacityBinding.Get()
		opacityValueLabel.SetText(fmt.Sprintf("%.0f%%", v*100))
	}))

	// Refresh interval slider with live value label
	intervalBinding := binding.NewFloat()
	intervalBinding.Set(float64(s.config.RefreshInterval))
	intervalSlider := widget.NewSliderWithData(15, 300, intervalBinding)
	intervalSlider.Step = 5
	intervalValueLabel := widget.NewLabel(fmt.Sprintf("%ds", s.config.RefreshInterval))
	intervalBinding.AddListener(binding.NewDataListener(func() {
		v, _ := intervalBinding.Get()
		intervalValueLabel.SetText(fmt.Sprintf("%.0fs", v))
	}))

	displaySection := container.NewVBox(
		displayLabel,
		container.NewHBox(widget.NewLabel("Opacity"), layout.NewSpacer(), opacityValueLabel),
		opacitySlider,
		container.NewHBox(widget.NewLabel("Refresh interval"), layout.NewSpacer(), intervalValueLabel),
		intervalSlider,
	)

	// --- Visible Stats ---
	visLabel := widget.NewLabel("Visible Stats")
	visLabel.TextStyle = fyne.TextStyle{Bold: true}

	sessionCheck := widget.NewCheck("Session", func(checked bool) {
		s.config.VisibleStats.SessionUsage = checked
	})
	sessionCheck.SetChecked(s.config.VisibleStats.SessionUsage)

	weeklyCheck := widget.NewCheck("Weekly", func(checked bool) {
		s.config.VisibleStats.WeeklyUsage = checked
	})
	weeklyCheck.SetChecked(s.config.VisibleStats.WeeklyUsage)

	resetCheck := widget.NewCheck("Reset Timers", func(checked bool) {
		s.config.VisibleStats.ResetTime = checked
	})
	resetCheck.SetChecked(s.config.VisibleStats.ResetTime)

	visSection := container.NewVBox(
		visLabel,
		container.NewGridWithColumns(3, sessionCheck, weeklyCheck, resetCheck),
	)

	// --- Buttons ---
	saveBtn := widget.NewButton("Save", func() {
		opacity, _ := opacityBinding.Get()
		interval, _ := intervalBinding.Get()

		s.config.OverlayOpacity = opacity
		s.config.RefreshInterval = int(interval)

		if err := s.config.Save(); err != nil {
			dialog.ShowError(err, window)
			return
		}

		if s.onSave != nil {
			s.onSave()
		}

		dialog.ShowInformation("Saved", "Settings saved", window)
	})
	saveBtn.Importance = widget.HighImportance

	closeBtn := widget.NewButton("Close", func() {
		window.Close()
	})

	buttons := container.NewHBox(layout.NewSpacer(), saveBtn, closeBtn, layout.NewSpacer())

	// --- Layout ---
	content := container.NewVBox(
		authSection,
		widget.NewSeparator(),
		displaySection,
		widget.NewSeparator(),
		visSection,
		widget.NewSeparator(),
		buttons,
	)

	window.SetContent(container.NewPadded(content))
	window.Show()
}
