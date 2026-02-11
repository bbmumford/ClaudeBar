package api

import (
	"claudebar/internal/browser"
	"claudebar/internal/config"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
)

var (
	ErrAuthFailed = errors.New("authentication failed")
)

// claudeCodeCredentials represents the Claude Code CLI credentials file
type claudeCodeCredentials struct {
	ClaudeAiOauth *claudeOAuth `json:"claudeAiOauth"`
	OrgUUID       string       `json:"organizationUuid"`
}

type claudeOAuth struct {
	AccessToken      string `json:"accessToken"`
	RefreshToken     string `json:"refreshToken"`
	ExpiresAt        int64  `json:"expiresAt"` // Unix timestamp in milliseconds
	SubscriptionType string `json:"subscriptionType"`
}

// AuthManager handles authentication for the Claude API
type AuthManager struct {
	client          *Client
	cookieExtractor *browser.CookieExtractor
	config          *config.Config
}

// NewAuthManager creates a new authentication manager
func NewAuthManager(client *Client) *AuthManager {
	return &AuthManager{
		client:          client,
		cookieExtractor: browser.NewCookieExtractor(),
		config:          config.Get(),
	}
}

// Initialize sets up authentication from available sources
// Priority: 1) Saved config  2) Claude Code CLI credentials  3) Browser cookies
func (a *AuthManager) Initialize() error {
	// First, try to load from config
	if a.config.SessionKey != "" {
		a.client.SetSessionKey(a.config.SessionKey)
		if a.config.OrganizationID != "" {
			a.client.SetOrganizationID(a.config.OrganizationID)
		}

		// Verify the session is still valid
		if err := a.verifyAndFetchOrg(); err == nil {
			log.Println("Authenticated using saved session key")
			return nil
		}

		log.Println("Saved session key expired, trying other sources...")
	}

	// Try to get org UUID from Claude Code CLI (even though its OAuth token won't work as session key)
	if err := a.tryClaudeCodeCredentials(); err == nil {
		log.Println("Got organization UUID from Claude Code CLI")
	}

	// Try to extract session key from browser cookies (may fail on Chrome 127+ due to App-Bound Encryption)
	sessionKey, err := a.cookieExtractor.ExtractSessionKey()
	if err != nil {
		log.Printf("Failed to extract session key from browser: %v", err)
		log.Println("Please set session key in Settings (copy from browser DevTools > Application > Cookies > claude.ai > sessionKey)")
		return ErrAuthFailed
	}

	a.client.SetSessionKey(sessionKey)

	// Fetch organization ID
	if err := a.verifyAndFetchOrg(); err != nil {
		return err
	}

	// Save the new credentials
	if err := a.config.SetSessionKey(sessionKey); err != nil {
		log.Printf("Warning: failed to save session key: %v", err)
	}

	log.Println("Successfully authenticated from browser cookies")
	return nil
}

// tryClaudeCodeCredentials reads the organization UUID from the Claude Code CLI.
// Note: Claude Code OAuth tokens (sk-ant-oat01-*) are NOT valid web session keys.
// We only extract the organization UUID to avoid an extra API call.
func (a *AuthManager) tryClaudeCodeCredentials() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	credPath := filepath.Join(home, ".claude", ".credentials.json")
	data, err := os.ReadFile(credPath)
	if err != nil {
		return err
	}

	var creds claudeCodeCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return err
	}

	// Only extract org UUID - OAuth tokens don't work with the web API
	if creds.OrgUUID != "" {
		a.client.SetOrganizationID(creds.OrgUUID)
		if err := a.config.SetOrganizationID(creds.OrgUUID); err != nil {
			log.Printf("Warning: failed to save organization ID: %v", err)
		}
		log.Printf("Found organization UUID from Claude Code: %s", creds.OrgUUID)
		return nil
	}

	return errors.New("no organization UUID in Claude Code credentials")
}

// verifyAndFetchOrg verifies the session and fetches organization ID
func (a *AuthManager) verifyAndFetchOrg() error {
	orgs, err := a.client.FetchOrganizations()
	if err != nil {
		return err
	}

	if len(orgs) == 0 {
		return errors.New("no organizations found")
	}

	// Use the first organization
	orgID := orgs[0].ID
	a.client.SetOrganizationID(orgID)

	// Save org ID
	if err := a.config.SetOrganizationID(orgID); err != nil {
		log.Printf("Warning: failed to save organization ID: %v", err)
	}

	return nil
}

// SetManualSessionKey allows manual session key entry
func (a *AuthManager) SetManualSessionKey(key string) error {
	a.client.SetSessionKey(key)

	if err := a.verifyAndFetchOrg(); err != nil {
		a.client.SetSessionKey("")
		return err
	}

	if err := a.config.SetSessionKey(key); err != nil {
		log.Printf("Warning: failed to save session key: %v", err)
	}

	return nil
}

// RefreshFromBrowser attempts to refresh credentials from browser
func (a *AuthManager) RefreshFromBrowser() error {
	sessionKey, err := a.cookieExtractor.ExtractSessionKey()
	if err != nil {
		return err
	}

	a.client.SetSessionKey(sessionKey)

	if err := a.verifyAndFetchOrg(); err != nil {
		return err
	}

	if err := a.config.SetSessionKey(sessionKey); err != nil {
		log.Printf("Warning: failed to save session key: %v", err)
	}

	return nil
}

// IsAuthenticated checks if we have valid credentials
func (a *AuthManager) IsAuthenticated() bool {
	return a.client.HasCredentials()
}

// ClearCredentials removes saved credentials
func (a *AuthManager) ClearCredentials() error {
	a.client.SetSessionKey("")
	a.client.SetOrganizationID("")
	a.config.SessionKey = ""
	a.config.OrganizationID = ""
	return a.config.Save()
}
