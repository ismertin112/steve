package auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

var (
	cfgMu    sync.RWMutex
	baseURL  string
	username string
	password string

	httpClient = &http.Client{Timeout: 15 * time.Second}
)

// Configure sets credentials that will be used for subsequent login requests.
func Configure(url, user, pass string) {
	cfgMu.Lock()
	defer cfgMu.Unlock()

	baseURL = url
	username = user
	password = pass
}

// LoginAndGetSession performs a login request against the configured panel and
// returns the authenticated session cookie.
func LoginAndGetSession() (*http.Cookie, error) {
	cfgMu.RLock()
	url := baseURL
	user := username
	pass := password
	cfgMu.RUnlock()

	if url == "" || user == "" || pass == "" {
		return nil, errors.New("auth: credentials are not configured")
	}

	payload := loginRequest{Username: user, Password: pass}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(payload); err != nil {
		return nil, fmt.Errorf("auth: encode login payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url+"login", &buf)
	if err != nil {
		return nil, fmt.Errorf("auth: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auth: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("auth: login failed: %s", resp.Status)
	}

	cookies := resp.Cookies()
	if len(cookies) == 0 {
		return nil, errors.New("auth: session cookie not found")
	}

	for _, c := range cookies {
		if c.Name == "session" {
			return c, nil
		}
	}

	// Fall back to the first cookie if the panel uses a different name.
	return cookies[0], nil
}
