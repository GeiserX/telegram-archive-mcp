package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
)

// Client wraps all HTTP interaction with telegram-archive's REST API.
// It performs cookie-based authentication (POST /api/login) and
// transparently re-authenticates on 401 responses.
type Client struct {
	base   string
	hc     *http.Client
	user   string
	pass   string
	cookie string // cached viewer_auth cookie value
	mu     sync.Mutex
}

// New creates a Client for the given base URL and credentials.
func New(base, user, pass string) *Client {
	return &Client{
		base: base,
		hc:   &http.Client{},
		user: user,
		pass: pass,
	}
}

// login authenticates and caches the viewer_auth cookie.
func (c *Client) login() error {
	body, _ := json.Marshal(map[string]string{
		"username": c.user,
		"password": c.pass,
	})
	req, err := http.NewRequest("POST", c.base+"/api/login", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("login: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("login: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("login failed (%d): %s", resp.StatusCode, string(b))
	}

	for _, ck := range resp.Cookies() {
		if ck.Name == "viewer_auth" {
			c.cookie = ck.Value
			return nil
		}
	}

	return fmt.Errorf("login: viewer_auth cookie not found in response")
}

// do executes an authenticated request. On 401 it re-authenticates once and retries.
func (c *Client) do(req *http.Request, retry bool) ([]byte, error) {
	c.mu.Lock()
	if c.cookie == "" {
		if err := c.login(); err != nil {
			c.mu.Unlock()
			return nil, err
		}
	}
	cookie := c.cookie
	c.mu.Unlock()

	req.Header.Set("Cookie", "viewer_auth="+cookie)

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Re-authenticate on 401 (expired cookie), but only once.
	if resp.StatusCode == 401 && retry {
		c.mu.Lock()
		c.cookie = ""
		c.mu.Unlock()

		// Build a fresh request (body may have been consumed).
		newReq, err := http.NewRequest(req.Method, req.URL.String(), nil)
		if err != nil {
			return nil, err
		}
		return c.do(newReq, false)
	}

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("telegram-archive error %d: %s", resp.StatusCode, string(b))
	}
	return io.ReadAll(resp.Body)
}

func (c *Client) buildURL(path string, q url.Values) string {
	u, _ := url.Parse(c.base)
	u.Path = path
	if q != nil {
		u.RawQuery = q.Encode()
	}
	return u.String()
}

// --- Resource methods -------------------------------------------------------

// GetStats returns global backup statistics.
func (c *Client) GetStats() ([]byte, error) {
	req, _ := http.NewRequest("GET", c.buildURL("/api/stats", nil), nil)
	return c.do(req, true)
}

// GetChats returns a list of chats.
func (c *Client) GetChats(limit int) ([]byte, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	req, _ := http.NewRequest("GET", c.buildURL("/api/chats", q), nil)
	return c.do(req, true)
}

// GetFolders returns the list of chat folders.
func (c *Client) GetFolders() ([]byte, error) {
	req, _ := http.NewRequest("GET", c.buildURL("/api/folders", nil), nil)
	return c.do(req, true)
}

// GetHealth returns the health status.
func (c *Client) GetHealth() ([]byte, error) {
	req, _ := http.NewRequest("GET", c.buildURL("/api/health", nil), nil)
	return c.do(req, true)
}

// --- Tool methods -----------------------------------------------------------

// SearchMessages searches messages in a chat.
func (c *Client) SearchMessages(chatID, query string, limit int) ([]byte, error) {
	q := url.Values{}
	q.Set("search", query)
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	path := fmt.Sprintf("/api/chats/%s/messages", url.PathEscape(chatID))
	req, _ := http.NewRequest("GET", c.buildURL(path, q), nil)
	return c.do(req, true)
}

// GetMessages retrieves messages from a chat with pagination.
func (c *Client) GetMessages(chatID string, limit, offset int) ([]byte, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	if offset > 0 {
		q.Set("offset", fmt.Sprintf("%d", offset))
	}
	path := fmt.Sprintf("/api/chats/%s/messages", url.PathEscape(chatID))
	req, _ := http.NewRequest("GET", c.buildURL(path, q), nil)
	return c.do(req, true)
}

// GetPinnedMessages returns pinned messages for a chat.
func (c *Client) GetPinnedMessages(chatID string) ([]byte, error) {
	path := fmt.Sprintf("/api/chats/%s/pinned", url.PathEscape(chatID))
	req, _ := http.NewRequest("GET", c.buildURL(path, nil), nil)
	return c.do(req, true)
}

// GetMessagesByDate retrieves messages from a specific date.
func (c *Client) GetMessagesByDate(chatID, date, timezone string) ([]byte, error) {
	q := url.Values{}
	q.Set("date", date)
	if timezone != "" {
		q.Set("timezone", timezone)
	}
	path := fmt.Sprintf("/api/chats/%s/messages/by-date", url.PathEscape(chatID))
	req, _ := http.NewRequest("GET", c.buildURL(path, q), nil)
	return c.do(req, true)
}

// GetChatStats returns statistics for a specific chat.
func (c *Client) GetChatStats(chatID string) ([]byte, error) {
	path := fmt.Sprintf("/api/chats/%s/stats", url.PathEscape(chatID))
	req, _ := http.NewRequest("GET", c.buildURL(path, nil), nil)
	return c.do(req, true)
}

// GetTopics returns topics for a chat.
func (c *Client) GetTopics(chatID string) ([]byte, error) {
	path := fmt.Sprintf("/api/chats/%s/topics", url.PathEscape(chatID))
	req, _ := http.NewRequest("GET", c.buildURL(path, nil), nil)
	return c.do(req, true)
}

// RefreshStats triggers a forced recalculation of global stats.
func (c *Client) RefreshStats() ([]byte, error) {
	req, _ := http.NewRequest("POST", c.buildURL("/api/stats/refresh", nil), nil)
	return c.do(req, true)
}
