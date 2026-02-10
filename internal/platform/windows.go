//go:build windows

package platform

import (
	"fmt"
	"log"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

var (
	user32                   = syscall.NewLazyDLL("user32.dll")
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	procSetWindowPos         = user32.NewProc("SetWindowPos")
	procMoveWindow           = user32.NewProc("MoveWindow")
	procGetWindowRect        = user32.NewProc("GetWindowRect")
	procGetWindowLong        = user32.NewProc("GetWindowLongW")
	procSetWindowLong        = user32.NewProc("SetWindowLongW")
	procSetLayeredWindowAttr = user32.NewProc("SetLayeredWindowAttributes")
	procGetSystemMetrics     = user32.NewProc("GetSystemMetrics")
	procSystemParametersInfo = user32.NewProc("SystemParametersInfoW")
	procRegisterHotKey       = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey     = user32.NewProc("UnregisterHotKey")
	procGetMessage           = user32.NewProc("GetMessageW")
	procPostThreadMessage    = user32.NewProc("PostThreadMessageW")
	procGetCurrentThreadId   = kernel32.NewProc("GetCurrentThreadId")
	procGetLastInputInfo     = user32.NewProc("GetLastInputInfo")
	procGetTickCount         = kernel32.NewProc("GetTickCount")
	procFindWindow           = user32.NewProc("FindWindowW")
)

// Windows constants
const (
	HWND_TOPMOST   = ^uintptr(0) // -1
	HWND_NOTOPMOST = ^uintptr(1) // -2
	SWP_NOMOVE     = 0x0002
	SWP_NOSIZE     = 0x0001
	SWP_NOACTIVATE = 0x0010
	SWP_SHOWWINDOW = 0x0040

	WS_EX_LAYERED     = 0x00080000
	WS_EX_TRANSPARENT  = 0x00000020
	WS_EX_TOOLWINDOW   = 0x00000080
	WS_EX_TOPMOST      = 0x00000008

	LWA_ALPHA    = 0x00000002
	LWA_COLORKEY = 0x00000001

	SM_CXSCREEN = 0
	SM_CYSCREEN = 1

	SPI_GETWORKAREA = 0x0030

	WM_HOTKEY = 0x0312
	WM_QUIT   = 0x0012
)

// gwlExStyle is GWL_EXSTYLE (-20) as uintptr, computed at runtime to avoid overflow
var gwlExStyle = negativeToUintptr(-20)

func negativeToUintptr(v int32) uintptr {
	return uintptr(uint32(v))
}

// RECT structure for Windows API
type RECT struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

// MSG structure for Windows message loop
type MSG struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

// LASTINPUTINFO for idle detection
type LASTINPUTINFO struct {
	CbSize uint32
	DwTime uint32
}

// WindowsFeatures implements PlatformFeatures for Windows
type WindowsFeatures struct {
	mu             sync.Mutex
	hotkeyThreadID uint32
	stopHotkey     chan struct{}
	hotkeyRunning  bool
}

// NewWindowsFeatures creates a new Windows platform features instance
func NewWindowsFeatures() *WindowsFeatures {
	return &WindowsFeatures{
		stopHotkey: make(chan struct{}),
	}
}

// SetAlwaysOnTop sets the window to always be on top
func (w *WindowsFeatures) SetAlwaysOnTop(handle WindowHandle, onTop bool) error {
	var insertAfter uintptr
	if onTop {
		insertAfter = HWND_TOPMOST
	} else {
		insertAfter = HWND_NOTOPMOST
	}

	ret, _, err := procSetWindowPos.Call(
		uintptr(handle),
		insertAfter,
		0, 0, 0, 0,
		SWP_NOMOVE|SWP_NOSIZE|SWP_NOACTIVATE,
	)

	if ret == 0 {
		return fmt.Errorf("SetWindowPos failed: %w", err)
	}
	return nil
}

// SetTransparency sets the window transparency
func (w *WindowsFeatures) SetTransparency(handle WindowHandle, opacity float64) error {
	exStyle, _, _ := procGetWindowLong.Call(uintptr(handle), gwlExStyle)
	newStyle := exStyle | WS_EX_LAYERED

	_, _, err := procSetWindowLong.Call(uintptr(handle), gwlExStyle, newStyle)
	if err != nil && err.Error() != "The operation completed successfully." {
		// Continue anyway
	}

	alpha := byte(opacity * 255)
	ret, _, err := procSetLayeredWindowAttr.Call(
		uintptr(handle),
		0,
		uintptr(alpha),
		LWA_ALPHA,
	)

	if ret == 0 {
		return fmt.Errorf("SetLayeredWindowAttributes failed: %w", err)
	}
	return nil
}

// MoveWindowTo moves a window to the specified position, keeping its current size
func (w *WindowsFeatures) MoveWindowTo(handle WindowHandle, x, y int) error {
	// Get current window size
	var rect RECT
	ret, _, err := procGetWindowRect.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&rect)),
	)
	if ret == 0 {
		return fmt.Errorf("GetWindowRect failed: %w", err)
	}

	width := rect.Right - rect.Left
	height := rect.Bottom - rect.Top

	ret, _, err = procMoveWindow.Call(
		uintptr(handle),
		uintptr(x),
		uintptr(y),
		uintptr(width),
		uintptr(height),
		1, // bRepaint = TRUE
	)
	if ret == 0 {
		return fmt.Errorf("MoveWindow failed: %w", err)
	}
	return nil
}

// MoveAndResizeWindow moves and resizes a window
func (w *WindowsFeatures) MoveAndResizeWindow(handle WindowHandle, x, y, width, height int) error {
	ret, _, err := procMoveWindow.Call(
		uintptr(handle),
		uintptr(x),
		uintptr(y),
		uintptr(width),
		uintptr(height),
		1,
	)
	if ret == 0 {
		return fmt.Errorf("MoveWindow failed: %w", err)
	}
	return nil
}

// GetWindowRect returns the actual window position and size (including frame/title bar)
func (w *WindowsFeatures) GetWindowRect(handle WindowHandle) (x, y, width, height int, err error) {
	var rect RECT
	ret, _, callErr := procGetWindowRect.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&rect)),
	)
	if ret == 0 {
		return 0, 0, 0, 0, fmt.Errorf("GetWindowRect failed: %w", callErr)
	}
	return int(rect.Left), int(rect.Top),
		int(rect.Right - rect.Left), int(rect.Bottom - rect.Top), nil
}

// SetClickThrough makes the window ignore mouse clicks
func (w *WindowsFeatures) SetClickThrough(handle WindowHandle, clickThrough bool) error {
	exStyle, _, _ := procGetWindowLong.Call(uintptr(handle), gwlExStyle)

	var newStyle uintptr
	if clickThrough {
		newStyle = exStyle | WS_EX_TRANSPARENT | WS_EX_LAYERED
	} else {
		newStyle = exStyle &^ WS_EX_TRANSPARENT
	}

	_, _, err := procSetWindowLong.Call(uintptr(handle), gwlExStyle, newStyle)
	if err != nil && err.Error() != "The operation completed successfully." {
		return fmt.Errorf("SetWindowLong failed: %w", err)
	}
	return nil
}

// GetScreenSize returns the primary screen dimensions
func (w *WindowsFeatures) GetScreenSize() (width, height int) {
	cx, _, _ := procGetSystemMetrics.Call(SM_CXSCREEN)
	cy, _, _ := procGetSystemMetrics.Call(SM_CYSCREEN)
	return int(cx), int(cy)
}

// GetWorkArea returns the usable screen area (excluding taskbar)
func (w *WindowsFeatures) GetWorkArea() (x, y, width, height int) {
	var rect RECT
	procSystemParametersInfo.Call(
		SPI_GETWORKAREA,
		0,
		uintptr(unsafe.Pointer(&rect)),
		0,
	)
	return int(rect.Left), int(rect.Top), int(rect.Right - rect.Left), int(rect.Bottom - rect.Top)
}

// GetIdleSeconds returns the number of seconds since the last keyboard/mouse input
func (w *WindowsFeatures) GetIdleSeconds() int {
	var info LASTINPUTINFO
	info.CbSize = uint32(unsafe.Sizeof(info))

	ret, _, _ := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&info)))
	if ret == 0 {
		return 0 // Assume active on error
	}

	tickCount, _, _ := procGetTickCount.Call()
	idleMs := uint32(tickCount) - info.DwTime
	return int(idleMs / 1000)
}

// RegisterHotkey registers a global hotkey
func (w *WindowsFeatures) RegisterHotkey(id int, modifiers uint, keyCode uint) error {
	ret, _, err := procRegisterHotKey.Call(
		0,
		uintptr(id),
		uintptr(modifiers),
		uintptr(keyCode),
	)

	if ret == 0 {
		return fmt.Errorf("RegisterHotKey failed for id %d: %w", id, err)
	}
	return nil
}

// UnregisterHotkey removes a registered hotkey
func (w *WindowsFeatures) UnregisterHotkey(id int) error {
	ret, _, err := procUnregisterHotKey.Call(
		0,
		uintptr(id),
	)

	if ret == 0 {
		return fmt.Errorf("UnregisterHotKey failed for id %d: %w", id, err)
	}
	return nil
}

// SetupHotkeyListener sets up the hotkey message loop
func (w *WindowsFeatures) SetupHotkeyListener(callback func(id int)) error {
	w.mu.Lock()
	if w.hotkeyRunning {
		w.mu.Unlock()
		return nil
	}
	w.hotkeyRunning = true
	w.stopHotkey = make(chan struct{})
	w.mu.Unlock()

	go func() {
		// CRITICAL: Lock this goroutine to the OS thread.
		// RegisterHotKey and GetMessage must run on the same OS thread.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		threadID, _, _ := procGetCurrentThreadId.Call()
		w.mu.Lock()
		w.hotkeyThreadID = uint32(threadID)
		w.mu.Unlock()

		// Register hotkeys:
		//   Ctrl+Alt+Arrow       = edge snaps (left, right, top)
		//   Ctrl+Alt+Shift+Arrow = corner snaps (top-left, top-right, bottom-left, bottom-right)
		//   Ctrl+Alt+.           = toggle overlay
		hotkeys := []struct {
			id   int
			mods uint
			key  uint
			name string
		}{
			// Edge snaps
			{HotkeySnapLeft, ModCtrl | ModAlt, VK_LEFT, "Ctrl+Alt+Left (snap left)"},
			{HotkeySnapRight, ModCtrl | ModAlt, VK_RIGHT, "Ctrl+Alt+Right (snap right)"},
			{HotkeySnapTop, ModCtrl | ModAlt, VK_UP, "Ctrl+Alt+Up (snap top)"},
			// Corner snaps (Shift = push to corner)
			{HotkeySnapTopLeft, ModCtrl | ModAlt | ModShift, VK_LEFT, "Ctrl+Alt+Shift+Left (top-left)"},
			{HotkeySnapTopRight, ModCtrl | ModAlt | ModShift, VK_RIGHT, "Ctrl+Alt+Shift+Right (top-right)"},
			{HotkeySnapBottomLeft, ModCtrl | ModAlt | ModShift, VK_DOWN, "Ctrl+Alt+Shift+Down (bottom-left)"},
			{HotkeySnapBottomRight, ModCtrl | ModAlt, VK_DOWN, "Ctrl+Alt+Down (bottom-right)"},
			// Toggle overlay
			{HotkeyToggleOverlay, ModCtrl | ModAlt, VK_OEM_PERIOD, "Ctrl+Alt+. (toggle overlay)"},
		}

		for _, hk := range hotkeys {
			if err := w.RegisterHotkey(hk.id, hk.mods, hk.key); err != nil {
				log.Printf("Failed to register %s: %v", hk.name, err)
			} else {
				log.Printf("Registered hotkey: %s", hk.name)
			}
		}

		// Message loop — GetMessage blocks until a message arrives
		var msg MSG
		for {
			ret, _, _ := procGetMessage.Call(
				uintptr(unsafe.Pointer(&msg)),
				0, 0, 0,
			)

			// ret == 0 means WM_QUIT, ret == -1 means error
			if ret == 0 || int32(ret) == -1 {
				break
			}

			if msg.Message == WM_HOTKEY {
				hotkeyID := int(msg.WParam)
				callback(hotkeyID)
			}
		}

		// Cleanup hotkeys
		for _, id := range []int{
			HotkeySnapLeft, HotkeySnapRight, HotkeySnapTop,
			HotkeySnapTopLeft, HotkeySnapTopRight,
			HotkeySnapBottomLeft, HotkeySnapBottomRight,
			HotkeyToggleOverlay,
		} {
			w.UnregisterHotkey(id)
		}
		log.Println("Hotkey message loop exited")
	}()

	return nil
}

// StopHotkeyListener stops the hotkey message loop
func (w *WindowsFeatures) StopHotkeyListener() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.hotkeyRunning {
		return
	}

	close(w.stopHotkey)
	w.hotkeyRunning = false

	// Post WM_QUIT to the hotkey thread to unblock GetMessage
	if w.hotkeyThreadID != 0 {
		procPostThreadMessage.Call(
			uintptr(w.hotkeyThreadID),
			WM_QUIT,
			0,
			0,
		)
	}
}

// GetWindowHandle extracts the native window handle by title
func GetWindowHandle(title string) (WindowHandle, error) {
	titlePtr, _ := syscall.UTF16PtrFromString(title)

	hwnd, _, err := procFindWindow.Call(
		0,
		uintptr(unsafe.Pointer(titlePtr)),
	)

	if hwnd == 0 {
		return 0, fmt.Errorf("FindWindow failed: %w", err)
	}

	return WindowHandle(hwnd), nil
}

// Global instance
var Features = NewWindowsFeatures()
