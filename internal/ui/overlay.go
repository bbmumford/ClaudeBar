package ui

import (
	"image/color"
	"log"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"

	"claudebar/internal/api"
	"claudebar/internal/config"
	"claudebar/internal/platform"
)

const (
	overlayTitle     = "ClaudeBar Overlay"
	verticalWidth    = 420
	horizontalWidth  = 700
	horizontalHeight = 55
)

// OverlayWindow manages the floating usage overlay
type OverlayWindow struct {
	window   fyne.Window
	app      fyne.App
	config   *config.Config
	platform platform.PlatformFeatures

	// Layout
	position   platform.SnapPosition
	isVertical bool

	// Vertical layout widgets (Claude website style)
	sessionRow      *UsageRow
	weeklyRow       *UsageRow
	sessionResetText *canvas.Text // session reset countdown
	weeklyResetText  *canvas.Text // weekly reset countdown

	// Horizontal (compact) layout widgets
	compactSession *CompactUsageRow
	compactWeekly  *CompactUsageRow
	compactReset   *canvas.Text

	// Status text (loading / error)
	statusText *canvas.Text

	// State
	mu           sync.RWMutex
	visible      bool
	initialized  bool
	windowHandle platform.WindowHandle
}

// NewOverlayWindow creates a new overlay window
func NewOverlayWindow(app fyne.App) *OverlayWindow {
	return &OverlayWindow{
		app:        app,
		config:     config.Get(),
		platform:   platform.Features,
		position:   platform.SnapPosition(config.Get().OverlayPosition),
		isVertical: true,
	}
}

// Setup initializes the overlay window
func (o *OverlayWindow) Setup() error {
	o.window = o.app.NewWindow(overlayTitle)
	o.window.SetPadded(false)
	// Fixed size — window always fits content exactly.
	// Layout changes only via snap hotkeys (Ctrl+Alt+Arrow).
	o.window.SetFixedSize(true)

	// Create both layouts
	o.createVerticalWidgets()
	o.createCompactWidgets()

	// Determine initial layout based on snap position
	if o.position == platform.SnapTop {
		o.isVertical = false
	} else {
		o.isVertical = true
	}

	o.applyLayout()

	o.initialized = true
	return nil
}

// applyLayout sets the window content and resizes to fit content exactly.
func (o *OverlayWindow) applyLayout() {
	bg := canvas.NewRectangle(color.RGBA{32, 33, 35, 240})

	if o.isVertical {
		o.buildVerticalContent(bg)
		minSize := o.window.Content().MinSize()
		w := minSize.Width
		if w < float32(verticalWidth) {
			w = float32(verticalWidth)
		}
		h := minSize.Height
		if h < 50 {
			h = 200
		}
		o.window.Resize(fyne.NewSize(w, h))
		log.Printf("Layout applied: vertical (%.0fx%.0f)", w, h)
	} else {
		o.buildHorizontalContent(bg)
		minSize := o.window.Content().MinSize()
		// Use content min size — compact widgets have proper MinSize set
		w := minSize.Width
		h := minSize.Height
		if h < 30 {
			h = 30
		}
		o.window.Resize(fyne.NewSize(w, h))
		log.Printf("Layout applied: horizontal (%.0fx%.0f)", w, h)
	}
}

// createVerticalWidgets creates the full Claude-style vertical layout
func (o *OverlayWindow) createVerticalWidgets() {
	o.sessionRow = NewUsageRow("Current session")
	o.weeklyRow = NewUsageRow("All models")
	o.sessionResetText = canvas.NewText("", colorGray)
	o.sessionResetText.TextSize = 13
	o.sessionResetText.TextStyle = fyne.TextStyle{Bold: true}
	o.weeklyResetText = canvas.NewText("", colorGray)
	o.weeklyResetText.TextSize = 12
	o.statusText = canvas.NewText("Loading...", colorGray)
	o.statusText.TextSize = 13
	o.statusText.Alignment = fyne.TextAlignCenter
}

// createCompactWidgets creates the minimal horizontal layout
func (o *OverlayWindow) createCompactWidgets() {
	o.compactSession = NewCompactUsageRow("Session")
	o.compactWeekly = NewCompactUsageRow("Weekly")
	o.compactReset = canvas.NewText("", colorGray)
	o.compactReset.TextSize = 10
	o.compactReset.SetMinSize(fyne.NewSize(260, 12)) // ensure space for "Session Xh Xm | Weekly Xd Xh"
}

// buildVerticalContent builds the full Claude-style layout
func (o *OverlayWindow) buildVerticalContent(bg *canvas.Rectangle) {
	items := []fyne.CanvasObject{}

	// Show status text if visible (loading/error state)
	if o.statusText != nil && o.statusText.Text != "" {
		items = append(items, o.statusText)
	}

	// Current session section
	if o.config.IsStatVisible("session") {
		items = append(items, o.sessionRow.GetContainer())
		items = append(items, Separator())
	}

	// Weekly limits section header
	if o.config.IsStatVisible("weekly") {
		weeklyHeader := SectionHeader("Weekly limits")
		items = append(items, weeklyHeader)
		items = append(items, canvas.NewRectangle(color.Transparent)) // small spacer
		items = append(items, o.weeklyRow.GetContainer())
	}

	// Reset timers
	if o.config.IsStatVisible("reset") {
		items = append(items, Separator())
		if o.sessionResetText != nil {
			items = append(items, o.sessionResetText)
		}
		if o.weeklyResetText != nil {
			items = append(items, o.weeklyResetText)
		}
	}

	content := container.NewVBox(items...)
	padded := container.NewPadded(content)
	stack := container.NewStack(bg, padded)
	o.window.SetContent(stack)
}

// buildHorizontalContent builds the compact top-bar layout
func (o *OverlayWindow) buildHorizontalContent(bg *canvas.Rectangle) {
	items := []fyne.CanvasObject{}

	// Show status text if visible (loading/error state)
	if o.statusText != nil && o.statusText.Text != "" {
		items = append(items, o.statusText)
	}

	if o.config.IsStatVisible("session") {
		items = append(items, o.compactSession.GetContainer())
	}
	if o.config.IsStatVisible("weekly") {
		items = append(items, o.compactWeekly.GetContainer())
	}
	if o.config.IsStatVisible("reset") {
		items = append(items, o.compactReset)
	}

	row := container.NewHBox(items...)
	centered := container.NewCenter(row)
	padded := container.NewPadded(centered)
	stack := container.NewStack(bg, padded)
	o.window.SetContent(stack)
}

// Show displays the overlay window
func (o *OverlayWindow) Show() {
	o.mu.Lock()
	defer o.mu.Unlock()

	if !o.initialized {
		return
	}

	o.window.Show()
	o.visible = true

	// Apply Windows-specific features and snap to position after window is shown
	go func() {
		time.Sleep(200 * time.Millisecond)
		o.applyWindowFeatures()
		// Snap to configured position on first show
		time.Sleep(100 * time.Millisecond)
		fyne.Do(func() {
			o.snapToPosition(o.position)
		})
	}()
}

// Hide hides the overlay window
func (o *OverlayWindow) Hide() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.window.Hide()
	o.visible = false
}

// Toggle toggles overlay visibility
func (o *OverlayWindow) Toggle() {
	o.mu.RLock()
	visible := o.visible
	o.mu.RUnlock()

	if visible {
		o.Hide()
	} else {
		o.Show()
	}
}

// IsVisible returns current visibility state
func (o *OverlayWindow) IsVisible() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.visible
}

// applyWindowFeatures applies Windows-specific features
func (o *OverlayWindow) applyWindowFeatures() {
	handle, err := platform.GetWindowHandle(overlayTitle)
	if err != nil {
		log.Printf("Failed to get window handle: %v", err)
		return
	}
	o.windowHandle = handle

	if err := o.platform.SetAlwaysOnTop(handle, true); err != nil {
		log.Printf("Failed to set always on top: %v", err)
	}

	opacity := o.config.OverlayOpacity
	if opacity <= 0 || opacity > 1 {
		opacity = 0.85
	}
	if err := o.platform.SetTransparency(handle, opacity); err != nil {
		log.Printf("Failed to set transparency: %v", err)
	}

	log.Printf("Window features applied (handle: %v, opacity: %.2f)", handle, opacity)
}

// SnapTo snaps the overlay to a screen position
func (o *OverlayWindow) SnapTo(pos platform.SnapPosition) {
	o.mu.Lock()
	o.position = pos
	o.mu.Unlock()

	// Switch layout based on position
	o.isVertical = (pos != platform.SnapTop)

	// Rebuild content and resize window to fit exactly
	o.applyLayout()

	o.snapToPosition(pos)
	o.config.SetOverlayPosition(string(pos))
}

// windowSize returns the actual window frame size via platform API.
// Falls back to Fyne content size + estimated frame if handle unavailable.
func (o *OverlayWindow) windowSize() (int, int) {
	// Try to get actual window rect from OS (includes title bar + borders)
	if o.windowHandle != 0 {
		_, _, w, h, err := o.platform.GetWindowRect(o.windowHandle)
		if err == nil && w > 0 && h > 0 {
			return w, h
		}
	}
	// Fallback: use fixed layout sizes
	if o.isVertical {
		h := 200
		if o.window.Content() != nil {
			if mh := int(o.window.Content().MinSize().Height); mh > 0 {
				h = mh
			}
		}
		return verticalWidth, h
	}
	return horizontalWidth, horizontalHeight
}

// snapToPosition moves the window to the snap position using Windows API
func (o *OverlayWindow) snapToPosition(pos platform.SnapPosition) {
	workX, workY, workW, workH := o.platform.GetWorkArea()
	cw, ch := o.windowSize()

	var x, y, w, h int

	switch pos {
	case platform.SnapLeft:
		w, h = cw, ch
		x = workX
		y = workY + (workH-h)/2
	case platform.SnapRight:
		w, h = cw, ch
		x = workX + workW - w
		y = workY + (workH-h)/2
	case platform.SnapTop:
		w, h = cw, ch
		x = workX + (workW-w)/2
		y = workY
	case platform.SnapTopLeft:
		w, h = cw, ch
		x = workX
		y = workY
	case platform.SnapTopRight:
		w, h = cw, ch
		x = workX + workW - w
		y = workY
	case platform.SnapBottomLeft:
		w, h = cw, ch
		x = workX
		y = workY + workH - h
	case platform.SnapBottomRight:
		w, h = cw, ch
		x = workX + workW - w
		y = workY + workH - h
	default:
		w, h = cw, ch
		if o.config.OverlayX >= 0 && o.config.OverlayY >= 0 {
			x = o.config.OverlayX
			y = o.config.OverlayY
		} else {
			x = workX + (workW-w)/2
			y = workY + (workH-h)/2
		}
	}

	log.Printf("Snapping to %s at (%d, %d) size %dx%d", pos, x, y, w, h)

	// Actually move the window using Windows API
	if o.windowHandle != 0 {
		if err := o.platform.MoveAndResizeWindow(o.windowHandle, x, y, w, h); err != nil {
			log.Printf("Failed to move window: %v", err)
		}
	}
	o.config.SetOverlayCoords(x, y)
}

// SetStatus updates the status text shown in the overlay (e.g. "Loading...", "Auth failed")
func (o *OverlayWindow) SetStatus(text string) {
	if o.statusText != nil {
		changed := o.statusText.Text != text
		o.statusText.Text = text
		o.statusText.Refresh()
		// Rebuild layout when status appears/disappears (changes window content)
		if changed && o.initialized {
			o.applyLayout()
			o.snapToPosition(o.position)
		}
	}
}

// UpdateUsage updates the displayed usage data
func (o *OverlayWindow) UpdateUsage(data *api.UsageData) {
	if data == nil || !o.initialized {
		return
	}

	// Clear loading/status text once we have data
	if o.statusText != nil && o.statusText.Text != "" {
		o.statusText.Text = ""
		o.statusText.Refresh()
	}

	// Update vertical layout widgets (Utilization is already 0-100)
	if o.sessionRow != nil {
		o.sessionRow.Update("Current session", data.FiveHour.Utilization, data.FiveHour.ResetsAt)
	}
	if o.weeklyRow != nil {
		o.weeklyRow.Update("All models", data.SevenDay.Utilization, time.Time{})
		o.weeklyRow.UpdateResetAbsolute(data.SevenDay.ResetsAt)
	}
	// Update reset timers
	if o.sessionResetText != nil && !data.FiveHour.ResetsAt.IsZero() {
		o.sessionResetText.Text = "Session resets in " + api.TimeUntilReset(data.FiveHour.ResetsAt)
		o.sessionResetText.Refresh()
	}
	if o.weeklyResetText != nil && !data.SevenDay.ResetsAt.IsZero() {
		o.weeklyResetText.Text = "Weekly resets in " + api.TimeUntilReset(data.SevenDay.ResetsAt)
		o.weeklyResetText.Refresh()
	}

	// Update compact layout widgets
	if o.compactSession != nil {
		o.compactSession.Update(data.FiveHour.Utilization)
	}
	if o.compactWeekly != nil {
		o.compactWeekly.Update(data.SevenDay.Utilization)
	}
	if o.compactReset != nil {
		resetText := ""
		if !data.FiveHour.ResetsAt.IsZero() {
			resetText = "Session " + api.TimeUntilReset(data.FiveHour.ResetsAt)
		}
		if !data.SevenDay.ResetsAt.IsZero() {
			if resetText != "" {
				resetText += " | "
			}
			resetText += "Weekly " + api.TimeUntilReset(data.SevenDay.ResetsAt)
		}
		o.compactReset.Text = resetText
		o.compactReset.Refresh()
	}

	// Rebuild layout so window resizes to fit new content
	o.applyLayout()
	o.snapToPosition(o.position)
}

// SetOpacity updates the overlay transparency
func (o *OverlayWindow) SetOpacity(opacity float64) {
	o.config.OverlayOpacity = opacity
	o.config.Save()
	if o.windowHandle != 0 {
		o.platform.SetTransparency(o.windowHandle, opacity)
	}
}

// GetWindow returns the underlying Fyne window
func (o *OverlayWindow) GetWindow() fyne.Window {
	return o.window
}
