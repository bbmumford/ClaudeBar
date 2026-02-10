//go:build linux

package platform

import (
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// LinuxFeatures implements PlatformFeatures for Linux using xdotool/wmctrl
type LinuxFeatures struct {
	mu             sync.Mutex
	hotkeyRunning  bool
	stopHotkey     chan struct{}
}

// NewLinuxFeatures creates a new Linux platform features instance
func NewLinuxFeatures() *LinuxFeatures {
	return &LinuxFeatures{
		stopHotkey: make(chan struct{}),
	}
}

// SetAlwaysOnTop sets the window to always be on top using wmctrl
func (l *LinuxFeatures) SetAlwaysOnTop(handle WindowHandle, onTop bool) error {
	// Use wmctrl to toggle always-on-top by window title
	// handle is unused on Linux; we find by title
	action := "add"
	if !onTop {
		action = "remove"
	}
	cmd := exec.Command("wmctrl", "-r", ":ACTIVE:", "-b", action+",above")
	if err := cmd.Run(); err != nil {
		// Try xdotool as fallback
		if onTop {
			cmd = exec.Command("xdotool", "getactivewindow", "set_window", "--overrideredirect", "1")
		} else {
			cmd = exec.Command("xdotool", "getactivewindow", "set_window", "--overrideredirect", "0")
		}
		if err2 := cmd.Run(); err2 != nil {
			log.Printf("SetAlwaysOnTop: wmctrl and xdotool both failed: %v / %v", err, err2)
			return fmt.Errorf("always-on-top not available (install wmctrl or xdotool)")
		}
	}
	return nil
}

// SetTransparency sets window transparency using xdotool + xprop
func (l *LinuxFeatures) SetTransparency(handle WindowHandle, opacity float64) error {
	// X11 transparency via _NET_WM_WINDOW_OPACITY
	alpha := uint32(opacity * 0xFFFFFFFF)
	cmd := exec.Command("xprop", "-id",
		fmt.Sprintf("0x%x", handle),
		"-f", "_NET_WM_WINDOW_OPACITY", "32c",
		"-set", "_NET_WM_WINDOW_OPACITY", fmt.Sprintf("%d", alpha))
	if err := cmd.Run(); err != nil {
		log.Printf("SetTransparency: xprop failed: %v", err)
		return fmt.Errorf("transparency not available (install xprop/x11-utils)")
	}
	return nil
}

// SetClickThrough is not easily supported on Linux without compositor-specific APIs
func (l *LinuxFeatures) SetClickThrough(handle WindowHandle, clickThrough bool) error {
	log.Println("SetClickThrough: not supported on Linux")
	return nil
}

// MoveWindowTo moves a window to a position
func (l *LinuxFeatures) MoveWindowTo(handle WindowHandle, x, y int) error {
	cmd := exec.Command("xdotool", "windowmove",
		fmt.Sprintf("%d", handle), fmt.Sprintf("%d", x), fmt.Sprintf("%d", y))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("xdotool windowmove failed: %w", err)
	}
	return nil
}

// GetWindowRect returns the window position and size
func (l *LinuxFeatures) GetWindowRect(handle WindowHandle) (x, y, width, height int, err error) {
	out, cmdErr := exec.Command("xdotool", "getwindowgeometry", "--shell", fmt.Sprintf("%d", handle)).Output()
	if cmdErr != nil {
		return 0, 0, 0, 0, fmt.Errorf("xdotool getwindowgeometry failed: %w", cmdErr)
	}
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		val, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
		switch parts[0] {
		case "X":
			x = val
		case "Y":
			y = val
		case "WIDTH":
			width = val
		case "HEIGHT":
			height = val
		}
	}
	return x, y, width, height, nil
}

// MoveAndResizeWindow moves and resizes a window
func (l *LinuxFeatures) MoveAndResizeWindow(handle WindowHandle, x, y, width, height int) error {
	cmd := exec.Command("xdotool", "windowmove", "--sync",
		fmt.Sprintf("%d", handle), fmt.Sprintf("%d", x), fmt.Sprintf("%d", y))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("xdotool windowmove failed: %w", err)
	}
	cmd = exec.Command("xdotool", "windowsize", "--sync",
		fmt.Sprintf("%d", handle), fmt.Sprintf("%d", width), fmt.Sprintf("%d", height))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("xdotool windowsize failed: %w", err)
	}
	return nil
}

// GetScreenSize returns primary screen dimensions via xdpyinfo
func (l *LinuxFeatures) GetScreenSize() (width, height int) {
	out, err := exec.Command("xdpyinfo").Output()
	if err != nil {
		return 1920, 1080
	}
	// Parse "dimensions: 1920x1080 pixels"
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "dimensions:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				dim := strings.Split(parts[1], "x")
				if len(dim) == 2 {
					w, _ := strconv.Atoi(dim[0])
					h, _ := strconv.Atoi(dim[1])
					if w > 0 && h > 0 {
						return w, h
					}
				}
			}
		}
	}
	return 1920, 1080
}

// GetWorkArea returns usable screen area (excluding panels/taskbars)
func (l *LinuxFeatures) GetWorkArea() (x, y, width, height int) {
	// Try _NET_WORKAREA via xprop
	out, err := exec.Command("xprop", "-root", "_NET_WORKAREA").Output()
	if err == nil {
		// Format: "_NET_WORKAREA(CARDINAL) = 0, 0, 1920, 1040, ..."
		s := string(out)
		if idx := strings.Index(s, "="); idx != -1 {
			vals := strings.Split(strings.TrimSpace(s[idx+1:]), ",")
			if len(vals) >= 4 {
				x, _ = strconv.Atoi(strings.TrimSpace(vals[0]))
				y, _ = strconv.Atoi(strings.TrimSpace(vals[1]))
				width, _ = strconv.Atoi(strings.TrimSpace(vals[2]))
				height, _ = strconv.Atoi(strings.TrimSpace(vals[3]))
				if width > 0 && height > 0 {
					return x, y, width, height
				}
			}
		}
	}
	// Fallback to full screen
	w, h := l.GetScreenSize()
	return 0, 0, w, h
}

// GetIdleSeconds returns seconds since last user input via xprintidle
func (l *LinuxFeatures) GetIdleSeconds() int {
	out, err := exec.Command("xprintidle").Output()
	if err != nil {
		return 0
	}
	ms, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0
	}
	return ms / 1000
}

// RegisterHotkey registers a global hotkey (stub - requires X11 keygrab)
func (l *LinuxFeatures) RegisterHotkey(id int, modifiers uint, keyCode uint) error {
	log.Printf("RegisterHotkey: global hotkeys require xdotool or custom X11 keygrab (id=%d)", id)
	return nil
}

// UnregisterHotkey removes a registered hotkey
func (l *LinuxFeatures) UnregisterHotkey(id int) error {
	return nil
}

// SetupHotkeyListener sets up hotkey listening
// On Linux, global hotkeys require either xbindkeys or X11 XGrabKey.
// For now, this is a basic implementation that doesn't support global hotkeys.
func (l *LinuxFeatures) SetupHotkeyListener(callback func(id int)) error {
	l.mu.Lock()
	if l.hotkeyRunning {
		l.mu.Unlock()
		return nil
	}
	l.hotkeyRunning = true
	l.stopHotkey = make(chan struct{})
	l.mu.Unlock()

	log.Println("Global hotkeys not yet implemented on Linux (use xbindkeys for manual setup)")
	_ = runtime.Version() // avoid unused import
	return nil
}

// StopHotkeyListener stops the hotkey listener
func (l *LinuxFeatures) StopHotkeyListener() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.hotkeyRunning {
		return
	}
	close(l.stopHotkey)
	l.hotkeyRunning = false
}

// GetWindowHandle finds a window by title using xdotool
func GetWindowHandle(title string) (WindowHandle, error) {
	out, err := exec.Command("xdotool", "search", "--name", title).Output()
	if err != nil {
		return 0, fmt.Errorf("xdotool search failed: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return 0, fmt.Errorf("window not found: %s", title)
	}
	id, err := strconv.ParseUint(lines[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid window id: %s", lines[0])
	}
	return WindowHandle(id), nil
}

// Global instance
var Features = NewLinuxFeatures()
