//go:build linux

package browser

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

// NewCookieExtractor creates a new cookie extractor with Linux paths
func NewCookieExtractor() *CookieExtractor {
	home, _ := os.UserHomeDir()

	// Use XDG paths
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		configDir = filepath.Join(home, ".config")
	}
	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		dataDir = filepath.Join(home, ".local", "share")
	}

	return &CookieExtractor{
		homeDir:      home,
		localAppData: configDir,
		appData:      dataDir,
	}
}

// chromiumBrowsers returns Chromium browser paths on Linux
func (c *CookieExtractor) chromiumBrowsers() []browserInfo {
	return []browserInfo{
		{"Chrome", filepath.Join(c.localAppData, "google-chrome")},
		{"Chromium", filepath.Join(c.localAppData, "chromium")},
		{"Brave", filepath.Join(c.localAppData, "BraveSoftware", "Brave-Browser")},
		{"Edge", filepath.Join(c.localAppData, "microsoft-edge")},
	}
}

// firefoxProfilesDir returns the Firefox profiles directory on Linux
func (c *CookieExtractor) firefoxProfilesDir() string {
	return filepath.Join(c.homeDir, ".mozilla", "firefox")
}

// openCookieDB tries to open a cookie database on Linux
func (c *CookieExtractor) openCookieDB(cookiePath string) (*sql.DB, func(), error) {
	noop := func() {}

	// Method 1: Open directly with immutable flag
	dsn := fmt.Sprintf("file:%s?mode=ro&immutable=1&_journal_mode=OFF", cookiePath)
	db, err := sql.Open("sqlite3", dsn)
	if err == nil {
		if err := db.Ping(); err == nil {
			log.Printf("    Opened DB with immutable flag")
			return db, noop, nil
		}
		db.Close()
	}

	// Method 2: Copy with cp
	tmpDir, err := os.MkdirTemp("", "claudebar_cookies_*")
	if err != nil {
		return nil, noop, err
	}
	tmpPath := filepath.Join(tmpDir, "Cookies")
	cleanup := func() { os.RemoveAll(tmpDir) }

	cmd := exec.Command("cp", cookiePath, tmpPath)
	if err := cmd.Run(); err == nil {
		db, err := sql.Open("sqlite3", tmpPath)
		if err == nil {
			log.Printf("    Opened DB via cp")
			return db, cleanup, nil
		}
	}

	// Method 3: Direct read
	data, err := os.ReadFile(cookiePath)
	if err == nil {
		tmpFile := filepath.Join(tmpDir, "Cookies_direct")
		if err := os.WriteFile(tmpFile, data, 0600); err == nil {
			db, err := sql.Open("sqlite3", tmpFile)
			if err == nil {
				log.Printf("    Opened DB via direct copy")
				return db, cleanup, nil
			}
		}
	}

	cleanup()
	return nil, noop, fmt.Errorf("all methods failed to open cookie DB at %s", cookiePath)
}

// decryptPlatformKey decrypts an encrypted key on Linux.
// Chrome on Linux uses libsecret (GNOME Keyring) or kwallet for key storage.
// This is a stub that returns an error - users should paste session key manually.
func decryptPlatformKey(encrypted []byte) ([]byte, error) {
	// TODO: Implement libsecret/kwallet integration for Chrome key decryption
	// For now, on Linux Chrome cookies can't be decrypted without keyring access
	return nil, errors.New("Chrome cookie decryption on Linux requires libsecret/GNOME Keyring (not yet implemented). Please paste session key manually in Settings")
}
