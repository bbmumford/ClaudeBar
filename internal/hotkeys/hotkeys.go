package hotkeys

import (
	"claudebar/internal/platform"
	"log"
	"sync"
)

// SnapCallback is called when a snap hotkey is pressed
type SnapCallback func(position platform.SnapPosition)

// ToggleCallback is called when the toggle overlay hotkey is pressed
type ToggleCallback func()

// Manager handles global hotkey registration and events
type Manager struct {
	platform       platform.PlatformFeatures
	snapCallback   SnapCallback
	toggleCallback ToggleCallback
	mu             sync.Mutex
	running        bool
}

// NewManager creates a new hotkey manager
func NewManager() *Manager {
	return &Manager{
		platform: platform.Features,
	}
}

// SetSnapCallback sets the callback for snap hotkeys
func (m *Manager) SetSnapCallback(callback SnapCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snapCallback = callback
}

// SetToggleCallback sets the callback for the toggle overlay hotkey
func (m *Manager) SetToggleCallback(callback ToggleCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.toggleCallback = callback
}

// Start begins listening for hotkeys
func (m *Manager) Start() error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return nil
	}
	m.running = true
	m.mu.Unlock()

	err := m.platform.SetupHotkeyListener(m.handleHotkey)
	if err != nil {
		log.Printf("Failed to setup hotkey listener: %v", err)
		return err
	}

	log.Println("Hotkey listener started")
	log.Println("  Ctrl+Alt+Left          -> Snap left")
	log.Println("  Ctrl+Alt+Right         -> Snap right")
	log.Println("  Ctrl+Alt+Up            -> Snap top")
	log.Println("  Ctrl+Alt+Down          -> Snap bottom-right")
	log.Println("  Ctrl+Alt+Shift+Left    -> Snap top-left")
	log.Println("  Ctrl+Alt+Shift+Right   -> Snap top-right")
	log.Println("  Ctrl+Alt+Shift+Down    -> Snap bottom-left")
	log.Println("  Ctrl+Alt+.             -> Toggle overlay")

	return nil
}

// Stop stops listening for hotkeys
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.platform.StopHotkeyListener()
	m.running = false
	log.Println("Hotkey listener stopped")
}

// handleHotkey processes hotkey events
func (m *Manager) handleHotkey(id int) {
	m.mu.Lock()
	snapCb := m.snapCallback
	toggleCb := m.toggleCallback
	m.mu.Unlock()

	// Handle toggle overlay
	if id == platform.HotkeyToggleOverlay {
		log.Println("Hotkey: Toggle overlay")
		if toggleCb != nil {
			toggleCb()
		}
		return
	}

	// Handle snap hotkeys
	if snapCb == nil {
		return
	}

	var position platform.SnapPosition

	switch id {
	case platform.HotkeySnapLeft:
		position = platform.SnapLeft
		log.Println("Hotkey: Snap left")
	case platform.HotkeySnapRight:
		position = platform.SnapRight
		log.Println("Hotkey: Snap right")
	case platform.HotkeySnapTop:
		position = platform.SnapTop
		log.Println("Hotkey: Snap top")
	case platform.HotkeySnapTopLeft:
		position = platform.SnapTopLeft
		log.Println("Hotkey: Snap top-left")
	case platform.HotkeySnapTopRight:
		position = platform.SnapTopRight
		log.Println("Hotkey: Snap top-right")
	case platform.HotkeySnapBottomLeft:
		position = platform.SnapBottomLeft
		log.Println("Hotkey: Snap bottom-left")
	case platform.HotkeySnapBottomRight:
		position = platform.SnapBottomRight
		log.Println("Hotkey: Snap bottom-right")
	default:
		log.Printf("Unknown hotkey ID: %d", id)
		return
	}

	snapCb(position)
}

// IsRunning returns whether the hotkey listener is active
func (m *Manager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}
