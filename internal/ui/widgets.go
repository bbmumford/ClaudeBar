package ui

import (
	"fmt"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"claudebar/internal/api"
)

// Claude website color palette
var (
	colorBg          = color.RGBA{32, 33, 35, 255}    // Dark background
	colorBarTrack    = color.RGBA{55, 57, 61, 255}    // Bar track
	colorBarFill     = color.RGBA{88, 140, 236, 255}  // Blue bar fill
	colorBarWarn     = color.RGBA{234, 179, 8, 255}   // Yellow warning
	colorBarCritical = color.RGBA{239, 68, 68, 255}   // Red critical
	colorWhite       = color.RGBA{237, 237, 237, 255} // Header text
	colorGray        = color.RGBA{156, 163, 175, 255} // Subtitle/reset text
	colorLightGray   = color.RGBA{180, 186, 194, 255} // Percentage text
	colorSeparator   = color.RGBA{55, 57, 61, 255}    // Divider line
)

// ProgressBar is a custom progress bar matching Claude's design
type ProgressBar struct {
	widget.BaseWidget
	percentage float64
	track      *canvas.Rectangle
	fill       *canvas.Rectangle
}

// NewProgressBar creates a progress bar
func NewProgressBar() *ProgressBar {
	p := &ProgressBar{}
	p.ExtendBaseWidget(p)
	return p
}

// SetValue sets the bar percentage (0-100)
func (p *ProgressBar) SetValue(pct float64) {
	p.percentage = pct
	if p.fill != nil {
		p.fill.FillColor = barColor(pct)
		p.fill.Refresh()
	}
	p.Refresh()
}

func (p *ProgressBar) CreateRenderer() fyne.WidgetRenderer {
	p.track = canvas.NewRectangle(colorBarTrack)
	p.track.CornerRadius = 5

	p.fill = canvas.NewRectangle(barColor(p.percentage))
	p.fill.CornerRadius = 5

	return &progressBarRenderer{bar: p}
}

type progressBarRenderer struct {
	bar *ProgressBar
}

func (r *progressBarRenderer) Layout(size fyne.Size) {
	r.bar.track.Resize(size)
	r.bar.track.Move(fyne.NewPos(0, 0))

	fillW := size.Width * float32(r.bar.percentage/100)
	if fillW < 0 {
		fillW = 0
	}
	if fillW > size.Width {
		fillW = size.Width
	}
	r.bar.fill.Resize(fyne.NewSize(fillW, size.Height))
	r.bar.fill.Move(fyne.NewPos(0, 0))
}

func (r *progressBarRenderer) MinSize() fyne.Size {
	return fyne.NewSize(80, 10)
}

func (r *progressBarRenderer) Refresh() {
	r.bar.fill.FillColor = barColor(r.bar.percentage)
	// Recalculate fill width on refresh (fixes bars not filling after data update)
	size := r.bar.track.Size()
	if size.Width > 0 {
		fillW := size.Width * float32(r.bar.percentage/100)
		if fillW < 0 {
			fillW = 0
		}
		if fillW > size.Width {
			fillW = size.Width
		}
		r.bar.fill.Resize(fyne.NewSize(fillW, size.Height))
	}
	r.bar.fill.Refresh()
	r.bar.track.Refresh()
}

func (r *progressBarRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bar.track, r.bar.fill}
}

func (r *progressBarRenderer) Destroy() {}

func barColor(pct float64) color.Color {
	if pct >= 90 {
		return colorBarCritical
	}
	if pct >= 75 {
		return colorBarWarn
	}
	return colorBarFill
}

// UsageRow displays a single usage metric matching Claude's website layout:
//
//	**Label**              [====bar====]   XX% used
//	Resets in X hr Y min
type UsageRow struct {
	container *fyne.Container

	headerText *canvas.Text
	resetText  *canvas.Text
	pctText    *canvas.Text
	bar        *ProgressBar
}

// NewUsageRow creates a usage row
func NewUsageRow(label string) *UsageRow {
	u := &UsageRow{}

	// Bold header
	u.headerText = canvas.NewText(label, colorWhite)
	u.headerText.TextSize = 14
	u.headerText.TextStyle = fyne.TextStyle{Bold: true}

	// Reset subtitle
	u.resetText = canvas.NewText("", colorGray)
	u.resetText.TextSize = 12

	// Percentage label
	u.pctText = canvas.NewText("0% used", colorLightGray)
	u.pctText.TextSize = 13

	// Progress bar
	u.bar = NewProgressBar()

	// Layout: header row with bar and percentage, then reset text below
	// Top row: [header] [spacer] [bar] [pct]
	barContainer := container.New(&fixedHeightLayout{height: 10}, u.bar)
	topRow := container.NewHBox(
		u.headerText,
		layout.NewSpacer(),
		container.New(&fixedWidthLayout{width: 180}, barContainer),
		u.pctText,
	)

	u.container = container.NewVBox(
		topRow,
		u.resetText,
	)

	return u
}

// Update refreshes the row with new data
func (u *UsageRow) Update(label string, pct float64, resetAt time.Time) {
	u.headerText.Text = label
	u.headerText.Refresh()

	u.pctText.Text = fmt.Sprintf("%.0f%% used", pct)
	u.pctText.Refresh()

	u.bar.SetValue(pct)

	if !resetAt.IsZero() {
		u.resetText.Text = "Resets in " + api.TimeUntilReset(resetAt)
		u.resetText.Refresh()
	}
}

// UpdateResetAbsolute sets the reset text to an absolute time
func (u *UsageRow) UpdateResetAbsolute(resetAt time.Time) {
	if resetAt.IsZero() {
		u.resetText.Text = ""
	} else {
		u.resetText.Text = "Resets " + resetAt.Format("Mon 3:04 PM")
	}
	u.resetText.Refresh()
}

// GetContainer returns the renderable container
func (u *UsageRow) GetContainer() *fyne.Container {
	return u.container
}

// SectionHeader creates a bold section header like "Weekly limits"
func SectionHeader(text string) *canvas.Text {
	t := canvas.NewText(text, colorWhite)
	t.TextSize = 15
	t.TextStyle = fyne.TextStyle{Bold: true}
	return t
}

// SectionSubtext creates gray subtext like "Learn more about usage limits"
func SectionSubtext(text string) *canvas.Text {
	t := canvas.NewText(text, colorGray)
	t.TextSize = 12
	return t
}

// Separator creates a thin horizontal divider line
func Separator() *canvas.Rectangle {
	sep := canvas.NewRectangle(colorSeparator)
	sep.SetMinSize(fyne.NewSize(0, 1))
	return sep
}

// fixedWidthLayout forces children to a fixed width
type fixedWidthLayout struct {
	width float32
}

func (l *fixedWidthLayout) MinSize(_ []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(l.width, 10)
}

func (l *fixedWidthLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Resize(fyne.NewSize(l.width, size.Height))
		o.Move(fyne.NewPos(0, 0))
	}
}

// fixedHeightLayout forces children to a fixed height (centered vertically)
type fixedHeightLayout struct {
	height float32
}

func (l *fixedHeightLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	w := float32(0)
	for _, o := range objects {
		w = fyne.Max(w, o.MinSize().Width)
	}
	return fyne.NewSize(w, l.height)
}

func (l *fixedHeightLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Resize(fyne.NewSize(size.Width, l.height))
		yOff := (size.Height - l.height) / 2
		o.Move(fyne.NewPos(0, yOff))
	}
}

// CompactUsageRow is a smaller version for horizontal/top snap
type CompactUsageRow struct {
	container *fyne.Container
	label     *canvas.Text
	pct       *canvas.Text
	bar       *ProgressBar
}

// NewCompactUsageRow creates a compact usage row for horizontal layout
func NewCompactUsageRow(labelStr string) *CompactUsageRow {
	c := &CompactUsageRow{}

	c.label = canvas.NewText(labelStr, colorWhite)
	c.label.TextSize = 11
	c.label.TextStyle = fyne.TextStyle{Bold: true}
	c.label.SetMinSize(fyne.NewSize(50, 14))

	c.pct = canvas.NewText("0%", colorLightGray)
	c.pct.TextSize = 11
	c.pct.SetMinSize(fyne.NewSize(30, 14))

	c.bar = NewProgressBar()

	barContainer := container.New(&fixedHeightLayout{height: 8}, c.bar)

	c.container = container.NewHBox(
		c.label,
		container.New(&fixedWidthLayout{width: 60}, barContainer),
		c.pct,
	)

	return c
}

// Update refreshes the compact row
func (c *CompactUsageRow) Update(pct float64) {
	c.pct.Text = fmt.Sprintf("%.0f%%", pct)
	c.pct.Refresh()
	c.bar.SetValue(pct)
}

// GetContainer returns the renderable container
func (c *CompactUsageRow) GetContainer() *fyne.Container {
	return c.container
}
