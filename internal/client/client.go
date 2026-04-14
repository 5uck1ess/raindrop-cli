package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/5uck1ess/raindrop-cli/internal/auth"
)

const (
	baseURL = "https://api.raindrop.io/rest/v1"
	// 120 req/min → one every 500ms, leave headroom at 600ms.
	minInterval = 600 * time.Millisecond
)

type Client struct {
	http        *http.Client
	token       string
	lastReq     time.Time
	LastHeaders http.Header
}

func New() (*Client, error) {
	tok, err := auth.Token()
	if err != nil {
		return nil, err
	}
	return &Client{
		http:  &http.Client{Timeout: 30 * time.Second},
		token: tok,
	}, nil
}

func (c *Client) throttle() {
	wait := time.Until(c.lastReq.Add(minInterval))
	if wait > 0 {
		time.Sleep(wait)
	}
	c.lastReq = time.Now()
}

func (c *Client) Do(method, path string, body, out any) error {
	c.throttle()

	var buf io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		buf = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, baseURL+path, buf)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()
	c.LastHeaders = resp.Header

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("raindrop api: %s %s → %d: %s", method, path, resp.StatusCode, string(data))
	}

	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("unmarshal: %w", err)
		}
	}
	return nil
}
