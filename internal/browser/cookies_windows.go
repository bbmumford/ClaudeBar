//go:build windows

package browser

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"
)

// Windows DPAPI functions
var (
	crypt32         = syscall.NewLazyDLL("crypt32.dll")
	procDecryptData = crypt32.NewProc("CryptUnprotectData")
)

type dataBlob struct {
	cbData uint32
	pbData *byte
}

// NewCookieExtractor creates a new cookie extractor with Windows paths
func NewCookieExtractor() *CookieExtractor {
	home, _ := os.UserHomeDir()
	return &CookieExtractor{
		homeDir:      home,
		localAppData: os.Getenv("LOCALAPPDATA"),
		appData:      os.Getenv("APPDATA"),
	}
}

// chromiumBrowsers returns Chromium browser paths on Windows
func (c *CookieExtractor) chromiumBrowsers() []browserInfo {
	return []browserInfo{
		{"Chrome", filepath.Join(c.localAppData, "Google", "Chrome", "User Data")},
		{"Edge", filepath.Join(c.localAppData, "Microsoft", "Edge", "User Data")},
		{"Brave", filepath.Join(c.localAppData, "BraveSoftware", "Brave-Browser", "User Data")},
	}
}

// firefoxProfilesDir returns the Firefox profiles directory on Windows
func (c *CookieExtractor) firefoxProfilesDir() string {
	return filepath.Join(c.appData, "Mozilla", "Firefox", "Profiles")
}

// openCookieDB tries multiple methods to open a locked cookie database on Windows
func (c *CookieExtractor) openCookieDB(cookiePath string) (*sql.DB, func(), error) {
	noop := func() {}

	// Method 1: Open directly with immutable flag (bypasses locks)
	dsn := fmt.Sprintf("file:%s?mode=ro&immutable=1&_journal_mode=OFF", cookiePath)
	db, err := sql.Open("sqlite3", dsn)
	if err == nil {
		if err := db.Ping(); err == nil {
			log.Printf("    Opened DB with immutable flag")
			return db, noop, nil
		}
		db.Close()
	}

	// Method 2: Copy with cmd /c copy (Windows native, sometimes bypasses locks)
	tmpDir, err := os.MkdirTemp("", "claudebar_cookies_*")
	if err != nil {
		return nil, noop, err
	}
	tmpPath := filepath.Join(tmpDir, "Cookies")
	cleanup := func() { os.RemoveAll(tmpDir) }

	cmd := exec.Command("cmd", "/c", "copy", "/Y", cookiePath, tmpPath)
	if err := cmd.Run(); err == nil {
		if _, err := os.Stat(tmpPath); err == nil {
			db, err := sql.Open("sqlite3", tmpPath)
			if err == nil {
				log.Printf("    Opened DB via cmd copy")
				return db, cleanup, nil
			}
		}
	}

	// Method 3: Direct os.ReadFile (works if browser is closed)
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

// decryptPlatformKey decrypts an encrypted key using Windows DPAPI
func decryptPlatformKey(encrypted []byte) ([]byte, error) {
	if len(encrypted) == 0 {
		return nil, errors.New("empty data")
	}

	var inBlob dataBlob
	inBlob.cbData = uint32(len(encrypted))
	inBlob.pbData = &encrypted[0]

	var outBlob dataBlob

	ret, _, err := procDecryptData.Call(
		uintptr(unsafe.Pointer(&inBlob)),
		0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&outBlob)),
	)

	if ret == 0 {
		return nil, fmt.Errorf("DPAPI decryption failed: %w", err)
	}

	decrypted := make([]byte, outBlob.cbData)
	copy(decrypted, unsafe.Slice(outBlob.pbData, outBlob.cbData))

	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	localFree := kernel32.NewProc("LocalFree")
	localFree.Call(uintptr(unsafe.Pointer(outBlob.pbData)))

	return decrypted, nil
}
