package panel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"vpn-bot/internal/panel/auth"
)

type Client struct {
	baseURL    string
	httpClient *http.Client

	mu      sync.RWMutex
	session *http.Cookie
}

type AddClientRequest struct {
	ID      int    `json:"id"`
	Email   string `json:"email"`
	LimitIP int    `json:"limitIp"`
	TotalGB int    `json:"totalGB"`
	Expiry  int64  `json:"expiryTime"`
	Enable  bool   `json:"enable"`
}

type AddClientResponse struct {
	Success bool   `json:"success"`
	Msg     string `json:"msg"`
	Obj     struct {
		ID string `json:"id"`
	} `json:"obj"`
}

type UpdateClientRequest struct {
	ID        string `json:"id"`
	Expiry    int64  `json:"expiryTime"`
	Operation string `json:"operation"`
}

type GenericResponse struct {
	Success bool   `json:"success"`
	Msg     string `json:"msg"`
}

type TrafficResponse struct {
	Success bool   `json:"success"`
	Msg     string `json:"msg"`
	Obj     []struct {
		ID     string `json:"id"`
		Expiry int64  `json:"expiryTime"`
		Enable bool   `json:"enable"`
	} `json:"obj"`
}

func New(baseURL string, session *http.Cookie) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 15 * time.Second},
		session:    session,
	}
}

func (c *Client) AddClient(ctx context.Context, userID int) (string, error) {
	reqBody := AddClientRequest{
		ID:      userID,
		Email:   fmt.Sprintf("user-%d@example.com", userID),
		LimitIP: 1,
		TotalGB: 0,
		Expiry:  time.Now().Add(30 * 24 * time.Hour).Unix(),
		Enable:  true,
	}
	var resp AddClientResponse
	if err := c.do(ctx, http.MethodPost, "xui/inbound/addClient", reqBody, &resp); err != nil {
		return "", err
	}
	if !resp.Success {
		return "", fmt.Errorf("panel error: %s", resp.Msg)
	}
	return resp.Obj.ID, nil
}

func (c *Client) UpdateClient(ctx context.Context, keyID string, days int) error {
	reqBody := UpdateClientRequest{
		ID:        keyID,
		Expiry:    time.Now().Add(time.Duration(days) * 24 * time.Hour).Unix(),
		Operation: "update",
	}
	return c.postGeneric(ctx, "xui/inbound/updateClient", reqBody)
}

func (c *Client) DelClient(ctx context.Context, keyID string) error {
	body := map[string]string{"id": keyID}
	return c.postGeneric(ctx, "xui/inbound/delClient", body)
}

func (c *Client) GetClientStatus(ctx context.Context, keyID string) (time.Time, error) {
	var resp TrafficResponse
	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("xui/inbound/getClientTraffics?id=%s", keyID), nil, &resp); err != nil {
		return time.Time{}, err
	}
	if !resp.Success || len(resp.Obj) == 0 {
		return time.Time{}, fmt.Errorf("client not found: %s", resp.Msg)
	}
	return time.Unix(resp.Obj[0].Expiry, 0), nil
}

func (c *Client) postGeneric(ctx context.Context, path string, body interface{}) error {
	var resp GenericResponse
	if err := c.do(ctx, http.MethodPost, path, body, &resp); err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("panel error: %s", resp.Msg)
	}
	return nil
}

func (c *Client) do(ctx context.Context, method, path string, body interface{}, dest interface{}) error {
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}

	for attempt := 0; attempt < 2; attempt++ {
		var reqBody *bytes.Reader
		if payload != nil {
			reqBody = bytes.NewReader(payload)
		}

		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
		if err != nil {
			return err
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		if session := c.getSession(); session != nil {
			req.AddCookie(session)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}

		if resp.StatusCode == http.StatusUnauthorized {
			resp.Body.Close()
			if err := c.refreshSession(); err != nil {
				return err
			}
			continue
		}

		if resp.StatusCode >= 300 {
			resp.Body.Close()
			return fmt.Errorf("panel request failed: %s", resp.Status)
		}

		if dest != nil {
			err = json.NewDecoder(resp.Body).Decode(dest)
			resp.Body.Close()
			return err
		}

		resp.Body.Close()
		return nil
	}

	return fmt.Errorf("panel request failed: unauthorized")
}

func (c *Client) getSession() *http.Cookie {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.session
}

func (c *Client) refreshSession() error {
	cookie, err := auth.LoginAndGetSession()
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.session = cookie
	c.mu.Unlock()
	return nil
}
