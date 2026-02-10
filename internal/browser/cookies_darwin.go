//go:build darwin

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

// NewCookieExtractor creates a new cookie extractor with macOS paths
func NewCookieExtractor() *CookieExtractor {
	home, _ := os.UserHomeDir()
	appSupport := filepath.Join(home, "Library", "Application Support")

	return &CookieExtractor{
		homeDir:      home,
		localAppData: appSupport,
		appData:      appSupport,
	}
}

// chromiumBrowsers returns Chromium browser paths on macOS
func (c *CookieExtractor) chromiumBrowsers() []browserInfo {
	return []browserInfo{
		{"Chrome", filepath.Join(c.localAppData, "Google", "Chrome")},
		{"Chromium", filepath.Join(c.localAppData, "Chromium")},
		{"Brave", filepath.Join(c.localAppData, "BraveSoftware", "Brave-Browser")},
		{"Edge", filepath.Join(c.localAppData, "Microsoft Edge")},
	}
}

// firefoxProfilesDir returns the Firefox profiles directory on macOS
func (c *CookieExtractor) firefoxProfilesDir() string {
	return filepath.Join(c.localAppData, "Firefox", "Profiles")
}

// openCookieDB tries to open a cookie database on macOS
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

// decryptPlatformKey decrypts an encrypted key on macOS.
// Chrome on macOS uses the Keychain for key storage.
// This is a stub that returns an error - users should paste session key manually.
func decryptPlatformKey(encrypted []byte) ([]byte, error) {
	// TODO: Implement Keychain integration for Chrome key decryption
	// The key is stored in Keychain under "Chrome Safe Storage"
	return nil, errors.New("Chrome cookie decryption on macOS requires Keychain access (not yet implemented). Please paste session key manually in Settings")
}
