package platform

// WindowHandle represents a platform-specific window handle
type WindowHandle uintptr

// SnapPosition represents where the window is snapped
type SnapPosition string

const (
	SnapNone        SnapPosition = "floating"
	SnapLeft        SnapPosition = "left"
	SnapRight       SnapPosition = "right"
	SnapTop         SnapPosition = "top"
	SnapTopLeft     SnapPosition = "top-left"
	SnapTopRight    SnapPosition = "top-right"
	SnapBottomLeft  SnapPosition = "bottom-left"
	SnapBottomRight SnapPosition = "bottom-right"
)

// PlatformFeatures defines the interface for platform-specific features.
// Each platform (Windows, Linux, macOS) must implement this interface.
type PlatformFeatures interface {
	// Window management
	SetAlwaysOnTop(handle WindowHandle, onTop bool) error
	SetTransparency(handle WindowHandle, opacity float64) error
	SetClickThrough(handle WindowHandle, clickThrough bool) error
	MoveWindowTo(handle WindowHandle, x, y int) error
	MoveAndResizeWindow(handle WindowHandle, x, y, width, height int) error
	GetWindowRect(handle WindowHandle) (x, y, width, height int, err error)

	// Screen info
	GetScreenSize() (width, height int)
	GetWorkArea() (x, y, width, height int)

	// Global hotkeys
	RegisterHotkey(id int, modifiers uint, keyCode uint) error
	UnregisterHotkey(id int) error
	SetupHotkeyListener(callback func(id int)) error
	StopHotkeyListener()

	// Idle detection
	GetIdleSeconds() int
}

// Hotkey modifiers
const (
	ModAlt   uint = 0x0001
	ModCtrl  uint = 0x0002
	ModShift uint = 0x0004
	ModWin   uint = 0x0008
)

// Virtual key codes
const (
	VK_LEFT       uint = 0x25
	VK_UP         uint = 0x26
	VK_RIGHT      uint = 0x27
	VK_DOWN       uint = 0x28
	VK_OEM_PERIOD uint = 0xBE // '.' key
)

// Hotkey IDs
const (
	HotkeySnapLeft        = 1
	HotkeySnapRight       = 2
	HotkeySnapTop         = 3
	HotkeySnapTopLeft     = 4
	HotkeySnapTopRight    = 5
	HotkeySnapBottomLeft  = 6
	HotkeySnapBottomRight = 7
	HotkeyToggleOverlay   = 8
)
