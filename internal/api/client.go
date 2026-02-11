package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

const (
	baseURL   = "https://claude.ai"
	userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
)

var (
	ErrNoSessionKey    = errors.New("no session key configured")
	ErrNoOrgID         = errors.New("no organization ID configured")
	ErrUnauthorized    = errors.New("unauthorized - session key may be invalid")
	ErrSessionExpired  = errors.New("session expired - please update session key")
	ErrRateLimited     = errors.New("rate limited - please wait before retrying")
	ErrAPIUnavailable  = errors.New("claude API is unavailable")
)

// APIError holds structured error details from the Claude API
type APIError struct {
	Type    string `json:"type"`
	Error   struct {
		Type    string `json:"type"`
		Message string `json:"message"`
		Details struct {
			ErrorCode string `json:"error_code"`
		} `json:"details"`
	} `json:"error"`
}

// Client handles communication with Claude's API
// Uses tls-client with Chrome TLS fingerprint to bypass Cloudflare
type Client struct {
	httpClient     tls_client.HttpClient
	sessionKey     string
	organizationID string
	mu             sync.RWMutex
	lastUsage      *UsageData
	lastFetch      time.Time
}

// NewClient creates a new API client with Chrome TLS fingerprint
func NewClient() *Client {
	options := []tls_client.HttpClientOption{
		tls_client.WithClientProfile(profiles.Chrome_131),
		tls_client.WithRandomTLSExtensionOrder(),
		tls_client.WithTimeoutSeconds(60),
	}

	tlsClient, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
	if err != nil {
		log.Printf("Warning: failed to create TLS client, falling back: %v", err)
		// Fallback - create with default profile
		tlsClient, _ = tls_client.NewHttpClient(tls_client.NewNoopLogger())
	}

	return &Client{
		httpClient: tlsClient,
	}
}

// SetSessionKey updates the session key
func (c *Client) SetSessionKey(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessionKey = key
}

// SetOrganizationID updates the organization ID
func (c *Client) SetOrganizationID(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.organizationID = id
}

// GetSessionKey returns the current session key
func (c *Client) GetSessionKey() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sessionKey
}

// GetOrganizationID returns the current organization ID
func (c *Client) GetOrganizationID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.organizationID
}

// HasCredentials checks if both session key and org ID are set
func (c *Client) HasCredentials() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sessionKey != "" && c.organizationID != ""
}

// FetchOrganizations retrieves the user's organizations (retries once on timeout)
func (c *Client) FetchOrganizations() ([]OrganizationInfo, error) {
	c.mu.RLock()
	sessionKey := c.sessionKey
	c.mu.RUnlock()

	if sessionKey == "" {
		return nil, ErrNoSessionKey
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			log.Printf("Retrying organizations fetch (attempt %d)...", attempt+1)
			time.Sleep(2 * time.Second)
		}

		orgs, err := c.fetchOrganizationsOnce(sessionKey)
		if err == nil {
			return orgs, nil
		}
		lastErr = err

		// Don't retry auth errors
		if err == ErrUnauthorized || err == ErrSessionExpired {
			return nil, err
		}
		log.Printf("Organizations fetch attempt %d failed: %v", attempt+1, err)
	}
	return nil, lastErr
}

func (c *Client) fetchOrganizationsOnce(sessionKey string) ([]OrganizationInfo, error) {
	req, err := http.NewRequest("GET", baseURL+"/api/organizations", nil)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req, sessionKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		log.Printf("Organizations API returned %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
		if apiErr := parseAPIError(body); apiErr == "account_session_invalid" {
			return nil, ErrSessionExpired
		}
		return nil, ErrUnauthorized
	}

	if resp.StatusCode != 200 {
		log.Printf("Organizations API returned %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var orgs []OrganizationInfo
	if err := json.Unmarshal(body, &orgs); err != nil {
		return nil, err
	}

	return orgs, nil
}

// FetchUsage retrieves the current usage data (retries once on transient errors)
// Uses endpoint: /api/organizations/{orgId}/usage
func (c *Client) FetchUsage() (*UsageData, error) {
	c.mu.RLock()
	sessionKey := c.sessionKey
	orgID := c.organizationID
	c.mu.RUnlock()

	if sessionKey == "" {
		return nil, ErrNoSessionKey
	}

	if orgID == "" {
		return nil, ErrNoOrgID
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			log.Printf("Retrying usage fetch (attempt %d)...", attempt+1)
			time.Sleep(2 * time.Second)
		}

		usage, err := c.fetchUsageOnce(sessionKey, orgID)
		if err == nil {
			return usage, nil
		}
		lastErr = err

		// Don't retry auth errors — they won't resolve on retry
		if errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrSessionExpired) {
			return nil, err
		}

		log.Printf("Usage fetch attempt %d failed: %v", attempt+1, err)
	}
	return nil, lastErr
}

func (c *Client) fetchUsageOnce(sessionKey, orgID string) (*UsageData, error) {
	url := fmt.Sprintf("%s/api/organizations/%s/usage", baseURL, orgID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req, sessionKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		// Parse the error body for specific error code
		if apiErr := parseAPIError(body); apiErr != "" {
			if apiErr == "account_session_invalid" {
				return nil, ErrSessionExpired
			}
			log.Printf("API error code: %s", apiErr)
		}
		return nil, ErrUnauthorized
	}

	if resp.StatusCode == 429 {
		return nil, ErrRateLimited
	}

	if resp.StatusCode >= 500 {
		return nil, ErrAPIUnavailable
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("usage endpoint returned: %d", resp.StatusCode)
	}

	// Parse the response
	var apiResponse UsageAPIResponse
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return nil, fmt.Errorf("failed to parse usage response: %w", err)
	}

	usage := apiResponse.ToUsageData()

	c.mu.Lock()
	c.lastUsage = usage
	c.lastFetch = time.Now()
	c.mu.Unlock()

	return usage, nil
}

// parseAPIError extracts the error_code from a Claude API error response.
// Returns empty string if the body can't be parsed.
func parseAPIError(body []byte) string {
	var apiErr APIError
	if err := json.Unmarshal(body, &apiErr); err != nil {
		return ""
	}
	return apiErr.Error.Details.ErrorCode
}

// setHeaders sets browser-like headers for API requests
func (c *Client) setHeaders(req *http.Request, sessionKey string) {
	req.Header = http.Header{
		"User-Agent":        {userAgent},
		"Accept":            {"application/json"},
		"Accept-Language":   {"en-US,en;q=0.9"},
		"Content-Type":      {"application/json"},
		"Origin":            {baseURL},
		"Referer":           {baseURL + "/"},
		"Sec-Fetch-Dest":    {"empty"},
		"Sec-Fetch-Mode":    {"cors"},
		"Sec-Fetch-Site":    {"same-origin"},
		"sec-ch-ua":         {`"Chromium";v="131", "Not_A Brand";v="24"`},
		"sec-ch-ua-mobile":  {"?0"},
		"sec-ch-ua-platform": {`"Windows"`},
		http.HeaderOrderKey: {
			"user-agent", "accept", "accept-language", "content-type",
			"cookie", "origin", "referer",
			"sec-fetch-dest", "sec-fetch-mode", "sec-fetch-site",
			"sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform",
		},
	}

	// Session key is sent as a cookie
	if strings.HasPrefix(sessionKey, "sk-ant-") {
		req.Header.Set("Cookie", fmt.Sprintf("sessionKey=%s", sessionKey))
	} else {
		req.Header.Set("Cookie", sessionKey)
	}
}

// GetLastUsage returns the cached usage data
func (c *Client) GetLastUsage() *UsageData {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastUsage
}

// GetLastFetchTime returns when usage was last fetched
func (c *Client) GetLastFetchTime() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastFetch
}
