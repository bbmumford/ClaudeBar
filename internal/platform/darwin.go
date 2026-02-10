//go:build darwin

package platform

import (
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// DarwinFeatures implements PlatformFeatures for macOS
type DarwinFeatures struct {
	mu            sync.Mutex
	hotkeyRunning bool
	stopHotkey    chan struct{}
}

// NewDarwinFeatures creates a new macOS platform features instance
func NewDarwinFeatures() *DarwinFeatures {
	return &DarwinFeatures{
		stopHotkey: make(chan struct{}),
	}
}

// SetAlwaysOnTop uses AppleScript to set window level
func (d *DarwinFeatures) SetAlwaysOnTop(handle WindowHandle, onTop bool) error {
	// macOS doesn't have a simple CLI for this; Fyne windows can use NSWindow level
	// For now, use AppleScript approach
	level := "0"
	if onTop {
		level = "1"
	}
	script := fmt.Sprintf(`
		tell application "System Events"
			set frontmost of every process whose unix id is %d to %s
		end tell`, handle, level)
	_ = script
	log.Println("SetAlwaysOnTop: limited support on macOS (window may not stay on top)")
	return nil
}

// SetTransparency sets window opacity via NSWindow
func (d *DarwinFeatures) SetTransparency(handle WindowHandle, opacity float64) error {
	// Requires CGO or AppleScript - stub for now
	log.Printf("SetTransparency: set to %.0f%% (native macOS transparency requires CGO)", opacity*100)
	return nil
}

// SetClickThrough is not easily supported without CGO
func (d *DarwinFeatures) SetClickThrough(handle WindowHandle, clickThrough bool) error {
	log.Println("SetClickThrough: not supported on macOS without CGO")
	return nil
}

// MoveWindowTo moves a window using AppleScript
func (d *DarwinFeatures) MoveWindowTo(handle WindowHandle, x, y int) error {
	script := fmt.Sprintf(`
		tell application "System Events"
			tell (first process whose frontmost is true)
				set position of window 1 to {%d, %d}
			end tell
		end tell`, x, y)
	cmd := exec.Command("osascript", "-e", script)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("AppleScript move failed: %w", err)
	}
	return nil
}

// GetWindowRect returns window position and size (stub on macOS)
func (d *DarwinFeatures) GetWindowRect(handle WindowHandle) (x, y, width, height int, err error) {
	return 0, 0, 0, 0, fmt.Errorf("GetWindowRect not implemented on macOS")
}

// MoveAndResizeWindow moves and resizes using AppleScript
func (d *DarwinFeatures) MoveAndResizeWindow(handle WindowHandle, x, y, width, height int) error {
	script := fmt.Sprintf(`
		tell application "System Events"
			tell (first process whose frontmost is true)
				set position of window 1 to {%d, %d}
				set size of window 1 to {%d, %d}
			end tell
		end tell`, x, y, width, height)
	cmd := exec.Command("osascript", "-e", script)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("AppleScript move/resize failed: %w", err)
	}
	return nil
}

// GetScreenSize returns the main display size
func (d *DarwinFeatures) GetScreenSize() (width, height int) {
	out, err := exec.Command("system_profiler", "SPDisplaysDataType").Output()
	if err != nil {
		return 1920, 1080
	}
	// Parse "Resolution: 2560 x 1440"
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Resolution:") {
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				w, _ := strconv.Atoi(parts[1])
				h, _ := strconv.Atoi(parts[3])
				if w > 0 && h > 0 {
					return w, h
				}
			}
		}
	}
	return 1920, 1080
}

// GetWorkArea returns the usable screen area (accounting for menu bar and dock)
func (d *DarwinFeatures) GetWorkArea() (x, y, width, height int) {
	// Use AppleScript to get visible frame
	script := `
		tell application "Finder"
			set screenBounds to bounds of window of desktop
			return screenBounds
		end tell`
	out, err := exec.Command("osascript", "-e", script).Output()
	if err == nil {
		parts := strings.Split(strings.TrimSpace(string(out)), ", ")
		if len(parts) >= 4 {
			x, _ = strconv.Atoi(parts[0])
			y, _ = strconv.Atoi(parts[1])
			right, _ := strconv.Atoi(parts[2])
			bottom, _ := strconv.Atoi(parts[3])
			return x, y, right - x, bottom - y
		}
	}
	// Fallback: full screen minus menu bar (approx 25px)
	w, h := d.GetScreenSize()
	return 0, 25, w, h - 25
}

// GetIdleSeconds returns seconds since last user input
func (d *DarwinFeatures) GetIdleSeconds() int {
	out, err := exec.Command("ioreg", "-c", "IOHIDSystem", "-d", "4").Output()
	if err != nil {
		return 0
	}
	// Parse "HIDIdleTime" = nanoseconds
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "HIDIdleTime") {
			parts := strings.Split(line, "=")
			if len(parts) >= 2 {
				val := strings.TrimSpace(parts[1])
				ns, err := strconv.ParseInt(val, 10, 64)
				if err == nil {
					return int(ns / 1_000_000_000)
				}
			}
		}
	}
	return 0
}

// RegisterHotkey - global hotkeys on macOS require Carbon API or CGO
func (d *DarwinFeatures) RegisterHotkey(id int, modifiers uint, keyCode uint) error {
	log.Printf("RegisterHotkey: global hotkeys require CGO on macOS (id=%d)", id)
	return nil
}

// UnregisterHotkey removes a registered hotkey
func (d *DarwinFeatures) UnregisterHotkey(id int) error {
	return nil
}

// SetupHotkeyListener sets up hotkey listening (stub on macOS)
func (d *DarwinFeatures) SetupHotkeyListener(callback func(id int)) error {
	d.mu.Lock()
	if d.hotkeyRunning {
		d.mu.Unlock()
		return nil
	}
	d.hotkeyRunning = true
	d.stopHotkey = make(chan struct{})
	d.mu.Unlock()

	log.Println("Global hotkeys not yet implemented on macOS (requires Carbon API)")
	return nil
}

// StopHotkeyListener stops the hotkey listener
func (d *DarwinFeatures) StopHotkeyListener() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.hotkeyRunning {
		return
	}
	close(d.stopHotkey)
	d.hotkeyRunning = false
}

// GetWindowHandle finds a window by title (stub on macOS)
func GetWindowHandle(title string) (WindowHandle, error) {
	// On macOS, we'd need CGWindowListCopyWindowInfo via CGO
	// For now return 0 (window features will degrade gracefully)
	log.Printf("GetWindowHandle: not implemented on macOS (title=%s)", title)
	return 0, fmt.Errorf("GetWindowHandle not implemented on macOS")
}

// Global instance
var Features = NewDarwinFeatures()
