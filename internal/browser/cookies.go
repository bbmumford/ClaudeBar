package browser

import (
	"crypto/aes"
	"crypto/cipher"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

var (
	ErrNoCookieFound   = errors.New("claude.ai session cookie not found")
	ErrDecryptFailed   = errors.New("failed to decrypt cookie value")
	ErrBrowserNotFound = errors.New("no supported browser found")
)

// browserInfo describes a browser path to search
type browserInfo struct {
	name string
	path string
}

// CookieExtractor handles browser cookie extraction.
// Platform-specific fields are initialized by NewCookieExtractor (per-platform file).
type CookieExtractor struct {
	homeDir string
	// Platform-specific path roots
	localAppData string // Windows: %LOCALAPPDATA%, Linux: ~/.config, macOS: ~/Library/Application Support
	appData      string // Windows: %APPDATA%, Linux: ~/.local/share, macOS: ~/Library/Application Support
}

// ExtractSessionKey attempts to extract the Claude session key from browsers
func (c *CookieExtractor) ExtractSessionKey() (string, error) {
	browsers := c.chromiumBrowsers()

	for _, browser := range browsers {
		if _, err := os.Stat(browser.path); os.IsNotExist(err) {
			continue
		}
		log.Printf("Trying %s at %s", browser.name, browser.path)

		key, err := c.extractFromChromium(browser.name, browser.path)
		if err == nil && key != "" {
			log.Printf("Found session key in %s", browser.name)
			return key, nil
		}
		if err != nil {
			log.Printf("%s: %v", browser.name, err)
		}
	}

	// Try Firefox
	key, err := c.extractFromFirefox()
	if err == nil && key != "" {
		log.Println("Found session key in Firefox")
		return key, nil
	}

	return "", ErrNoCookieFound
}

// extractFromChromium reads the session key from a Chromium-based browser
func (c *CookieExtractor) extractFromChromium(browserName, userDataPath string) (string, error) {
	profiles := []string{"Default", "Profile 1", "Profile 2", "Profile 3"}

	encKey, keyErr := c.getChromiumEncryptionKey(userDataPath)
	if keyErr != nil {
		log.Printf("%s: encryption key error: %v", browserName, keyErr)
	}

	for _, profile := range profiles {
		cookiePath := filepath.Join(userDataPath, profile, "Network", "Cookies")
		if _, err := os.Stat(cookiePath); os.IsNotExist(err) {
			cookiePath = filepath.Join(userDataPath, profile, "Cookies")
			if _, err := os.Stat(cookiePath); os.IsNotExist(err) {
				continue
			}
		}

		log.Printf("  Trying profile: %s", profile)

		key, err := c.readCookieDB(cookiePath, encKey)
		if err != nil {
			log.Printf("  %s/%s: %v", browserName, profile, err)
			continue
		}
		if key != "" {
			return key, nil
		}
	}

	return "", ErrNoCookieFound
}

// readCookieDB reads the claude.ai session key from a cookie database
func (c *CookieExtractor) readCookieDB(cookiePath string, encKey []byte) (string, error) {
	db, cleanup, err := c.openCookieDB(cookiePath)
	if err != nil {
		return "", err
	}
	defer cleanup()
	defer db.Close()

	query := `
		SELECT name, encrypted_value, value
		FROM cookies
		WHERE host_key LIKE '%claude.ai%'
		ORDER BY last_access_utc DESC
	`

	rows, err := db.Query(query)
	if err != nil {
		return "", fmt.Errorf("query error: %w", err)
	}
	defer rows.Close()

	var lastErr error
	for rows.Next() {
		var name string
		var encryptedValue []byte
		var plainValue string

		if err := rows.Scan(&name, &encryptedValue, &plainValue); err != nil {
			continue
		}

		log.Printf("    Found cookie: %s (encrypted=%d bytes, plain=%d bytes)",
			name, len(encryptedValue), len(plainValue))

		if plainValue != "" && isValidSessionKey(plainValue) {
			return plainValue, nil
		}

		if len(encryptedValue) > 0 && encKey != nil {
			decrypted, err := decryptChromeValue(encryptedValue, encKey)
			if err != nil {
				lastErr = err
				log.Printf("    Decrypt failed for %s: %v", name, err)
				continue
			}
			if isValidSessionKey(decrypted) {
				return decrypted, nil
			}
			if name == "sessionKey" && len(decrypted) > 20 {
				return decrypted, nil
			}
		}
	}

	if lastErr != nil {
		return "", lastErr
	}
	return "", ErrNoCookieFound
}

// extractFromFirefox reads the session key from Firefox cookies
func (c *CookieExtractor) extractFromFirefox() (string, error) {
	firefoxDir := c.firefoxProfilesDir()
	entries, err := os.ReadDir(firefoxDir)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if !strings.Contains(entry.Name(), ".default") && !strings.Contains(entry.Name(), "default-release") {
			continue
		}

		cookiePath := filepath.Join(firefoxDir, entry.Name(), "cookies.sqlite")
		if _, err := os.Stat(cookiePath); os.IsNotExist(err) {
			continue
		}

		dsn := fmt.Sprintf("file:%s?mode=ro&immutable=1", cookiePath)
		db, err := sql.Open("sqlite3", dsn)
		if err != nil {
			continue
		}
		defer db.Close()

		rows, err := db.Query(`
			SELECT name, value FROM moz_cookies
			WHERE host LIKE '%claude.ai%'
			AND (name = 'sessionKey' OR name LIKE '%session%')
		`)
		if err != nil {
			continue
		}
		defer rows.Close()

		for rows.Next() {
			var name, value string
			if err := rows.Scan(&name, &value); err != nil {
				continue
			}
			if isValidSessionKey(value) {
				return value, nil
			}
		}
	}

	return "", ErrNoCookieFound
}

// decryptChromeValue decrypts a Chrome cookie value
func decryptChromeValue(encryptedValue, key []byte) (string, error) {
	if len(encryptedValue) < 3 {
		return "", errors.New("encrypted value too short")
	}

	prefix := string(encryptedValue[:3])

	if prefix == "v20" {
		return "", errors.New("cookie uses Chrome App-Bound Encryption (v20) - third-party decryption not possible. Use Claude Code CLI or paste session key manually")
	}

	if prefix != "v10" && prefix != "v11" {
		decrypted, err := decryptPlatformKey(encryptedValue)
		if err != nil {
			return "", err
		}
		return string(decrypted), nil
	}

	if len(encryptedValue) < 3+12+16 {
		return "", errors.New("encrypted value too short for AES-GCM")
	}

	nonce := encryptedValue[3:15]
	ciphertext := encryptedValue[15:]

	decrypted, err := decryptAESGCM(key, nonce, ciphertext)
	if err != nil {
		return "", err
	}

	return string(decrypted), nil
}

// getChromiumEncryptionKey retrieves the encryption key from Local State
func (c *CookieExtractor) getChromiumEncryptionKey(userDataPath string) ([]byte, error) {
	localStatePath := filepath.Join(userDataPath, "Local State")

	data, err := os.ReadFile(localStatePath)
	if err != nil {
		return nil, err
	}

	keyStr := extractJSONValue(string(data), "encrypted_key")
	if keyStr == "" {
		return nil, errors.New("encrypted_key not found in Local State")
	}

	encryptedKey, err := base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		return nil, err
	}

	if len(encryptedKey) < 5 {
		return nil, errors.New("invalid encrypted key format")
	}
	encryptedKey = encryptedKey[5:] // Remove "DPAPI" prefix

	return decryptPlatformKey(encryptedKey)
}

// extractJSONValue is a simple JSON value extractor
func extractJSONValue(json, key string) string {
	search := `"` + key + `":"`
	start := strings.Index(json, search)
	if start == -1 {
		return ""
	}
	start += len(search)
	end := strings.Index(json[start:], `"`)
	if end == -1 {
		return ""
	}
	return json[start : start+end]
}

// isValidSessionKey checks if a string looks like a valid Claude session key
func isValidSessionKey(key string) bool {
	if strings.HasPrefix(key, "sk-ant-") {
		return true
	}
	parts := strings.Split(key, ".")
	if len(parts) == 3 && len(key) > 100 {
		return true
	}
	return false
}

// decryptAESGCM decrypts AES-GCM encrypted data
func decryptAESGCM(key, nonce, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("AES-GCM decryption failed: %w", err)
	}

	return plaintext, nil
}
