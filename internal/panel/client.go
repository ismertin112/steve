package panel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
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

func New(baseURL, token string) *Client {
	return &Client{
		baseURL:    baseURL,
		token:      token,
		httpClient: &http.Client{Timeout: 15 * time.Second},
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
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return err
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("panel request failed: %s", resp.Status)
	}

	if dest != nil {
		return json.NewDecoder(resp.Body).Decode(dest)
	}
	return nil
}
