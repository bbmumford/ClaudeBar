package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// Config holds all application settings
type Config struct {
	SessionKey      string         `json:"session_key,omitempty"`
	OrganizationID  string         `json:"organization_id,omitempty"`
	RefreshInterval int            `json:"refresh_interval"` // seconds
	OverlayEnabled  bool           `json:"overlay_enabled"`
	OverlayOpacity  float64        `json:"overlay_opacity"`
	OverlayPosition string         `json:"overlay_position"` // "left", "right", "top", "floating"
	OverlayX        int            `json:"overlay_x"`
	OverlayY        int            `json:"overlay_y"`
	VisibleStats    VisibleStats   `json:"visible_stats"`
	AutoStart       bool           `json:"auto_start"`
}

// VisibleStats controls which stats are shown
type VisibleStats struct {
	SessionUsage bool `json:"session_usage"`
	DailyUsage   bool `json:"daily_usage"`
	WeeklyUsage  bool `json:"weekly_usage"`
	ResetTime    bool `json:"reset_time"`
}

var (
	instance *Config
	once     sync.Once
	mu       sync.RWMutex
	configPath string
)

// Default returns the default configuration
func Default() *Config {
	return &Config{
		RefreshInterval: 60,
		OverlayEnabled:  true,
		OverlayOpacity:  0.85,
		OverlayPosition: "top",
		OverlayX:        -1,
		OverlayY:        -1,
		VisibleStats: VisibleStats{
			SessionUsage: true,
			DailyUsage:   true,
			WeeklyUsage:  true,
			ResetTime:    true,
		},
		AutoStart: false,
	}
}

// Get returns the singleton config instance
func Get() *Config {
	once.Do(func() {
		instance = Default()
		_ = instance.Load()
	})
	return instance
}

// getConfigPath returns the path to the config file.
// Uses platform-appropriate directories:
//   - Windows: %APPDATA%\ClaudeBar\config.json
//   - macOS:   ~/Library/Application Support/ClaudeBar/config.json
//   - Linux:   ~/.config/claudebar/config.json (XDG_CONFIG_HOME)
func getConfigPath() (string, error) {
	if configPath != "" {
		return configPath, nil
	}

	var dir string

	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		dir = filepath.Join(appData, "ClaudeBar")

	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, "Library", "Application Support", "ClaudeBar")

	default: // linux and others
		configHome := os.Getenv("XDG_CONFIG_HOME")
		if configHome == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			configHome = filepath.Join(home, ".config")
		}
		dir = filepath.Join(configHome, "claudebar")
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	configPath = filepath.Join(dir, "config.json")
	return configPath, nil
}

// Load reads the config from disk
func (c *Config) Load() error {
	mu.Lock()
	defer mu.Unlock()

	path, err := getConfigPath()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Use defaults
		}
		return err
	}

	return json.Unmarshal(data, c)
}

// Save writes the config to disk
func (c *Config) Save() error {
	mu.Lock()
	defer mu.Unlock()

	path, err := getConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// SetSessionKey updates the session key and saves
func (c *Config) SetSessionKey(key string) error {
	c.SessionKey = key
	return c.Save()
}

// SetOrganizationID updates the org ID and saves
func (c *Config) SetOrganizationID(id string) error {
	c.OrganizationID = id
	return c.Save()
}

// SetOverlayPosition updates position and saves
func (c *Config) SetOverlayPosition(pos string) error {
	c.OverlayPosition = pos
	return c.Save()
}

// SetOverlayCoords updates overlay coordinates
func (c *Config) SetOverlayCoords(x, y int) error {
	c.OverlayX = x
	c.OverlayY = y
	return c.Save()
}

// ToggleOverlay toggles overlay visibility
func (c *Config) ToggleOverlay() error {
	c.OverlayEnabled = !c.OverlayEnabled
	return c.Save()
}

// IsStatVisible checks if a stat type should be shown
func (c *Config) IsStatVisible(statType string) bool {
	switch statType {
	case "session":
		return c.VisibleStats.SessionUsage
	case "daily":
		return c.VisibleStats.DailyUsage
	case "weekly":
		return c.VisibleStats.WeeklyUsage
	case "reset":
		return c.VisibleStats.ResetTime
	default:
		return true
	}
}

// ToggleStat toggles visibility of a stat type
func (c *Config) ToggleStat(statType string) error {
	switch statType {
	case "session":
		c.VisibleStats.SessionUsage = !c.VisibleStats.SessionUsage
	case "daily":
		c.VisibleStats.DailyUsage = !c.VisibleStats.DailyUsage
	case "weekly":
		c.VisibleStats.WeeklyUsage = !c.VisibleStats.WeeklyUsage
	case "reset":
		c.VisibleStats.ResetTime = !c.VisibleStats.ResetTime
	}
	return c.Save()
}
