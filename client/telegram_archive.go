package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const maxResponseBody = 10 << 20 // 10 MB

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
		base: strings.TrimRight(base, "/"),
		hc:   &http.Client{Timeout: 30 * time.Second},
		user: user,
		pass: pass,
	}
}

// login authenticates and caches the viewer_auth cookie.
func (c *Client) login(ctx context.Context) error {
	body, err := json.Marshal(map[string]string{
		"username": c.user,
		"password": c.pass,
	})
	if err != nil {
		return fmt.Errorf("login: marshal body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.base+"/api/login", bytes.NewReader(body))
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
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
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

// doAuth executes an authenticated request. On 401 it re-authenticates once
// and retries. The body parameter is kept so retries can replay it (the
// original request body is consumed by the first attempt).
func (c *Client) doAuth(ctx context.Context, method, rawurl string, body []byte, retry bool) ([]byte, error) {
	c.mu.Lock()
	if c.cookie == "" {
		if err := c.login(ctx); err != nil {
			c.mu.Unlock()
			return nil, err
		}
	}
	cookie := c.cookie
	c.mu.Unlock()

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawurl, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.AddCookie(&http.Cookie{Name: "viewer_auth", Value: cookie})

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

		return c.doAuth(ctx, method, rawurl, body, false)
	}

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		msg := string(b)
		if len(msg) > 200 {
			cut := 200
			for cut > 0 && !utf8.RuneStart(msg[cut]) {
				cut--
			}
			msg = msg[:cut] + "..."
		}
		return nil, fmt.Errorf("telegram-archive error %d: %s", resp.StatusCode, msg)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > maxResponseBody {
		return nil, fmt.Errorf("telegram-archive response exceeds %d bytes", maxResponseBody)
	}
	return b, nil
}

func (c *Client) buildURL(path string, q url.Values) string {
	u := c.base + path
	if q != nil && len(q) > 0 {
		u += "?" + q.Encode()
	}
	return u
}

// --- Resource methods -------------------------------------------------------

// GetStats returns global backup statistics.
func (c *Client) GetStats(ctx context.Context) ([]byte, error) {
	return c.doAuth(ctx, "GET", c.buildURL("/api/stats", nil), nil, true)
}

// GetChats returns a list of chats.
func (c *Client) GetChats(ctx context.Context, limit int) ([]byte, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	return c.doAuth(ctx, "GET", c.buildURL("/api/chats", q), nil, true)
}

// GetFolders returns the list of chat folders.
func (c *Client) GetFolders(ctx context.Context) ([]byte, error) {
	return c.doAuth(ctx, "GET", c.buildURL("/api/folders", nil), nil, true)
}

// GetHealth returns the health status.
func (c *Client) GetHealth(ctx context.Context) ([]byte, error) {
	return c.doAuth(ctx, "GET", c.buildURL("/api/health", nil), nil, true)
}

// --- Tool methods -----------------------------------------------------------

// SearchMessages searches messages in a chat.
func (c *Client) SearchMessages(ctx context.Context, chatID, query string, limit int) ([]byte, error) {
	q := url.Values{}
	q.Set("search", query)
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	path := fmt.Sprintf("/api/chats/%s/messages", url.PathEscape(chatID))
	return c.doAuth(ctx, "GET", c.buildURL(path, q), nil, true)
}

// GetMessages retrieves messages from a chat with pagination.
func (c *Client) GetMessages(ctx context.Context, chatID string, limit, offset int) ([]byte, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	if offset > 0 {
		q.Set("offset", fmt.Sprintf("%d", offset))
	}
	path := fmt.Sprintf("/api/chats/%s/messages", url.PathEscape(chatID))
	return c.doAuth(ctx, "GET", c.buildURL(path, q), nil, true)
}

// GetPinnedMessages returns pinned messages for a chat.
func (c *Client) GetPinnedMessages(ctx context.Context, chatID string) ([]byte, error) {
	path := fmt.Sprintf("/api/chats/%s/pinned", url.PathEscape(chatID))
	return c.doAuth(ctx, "GET", c.buildURL(path, nil), nil, true)
}

// GetMessagesByDate retrieves messages from a specific date.
func (c *Client) GetMessagesByDate(ctx context.Context, chatID, date, timezone string) ([]byte, error) {
	q := url.Values{}
	q.Set("date", date)
	if timezone != "" {
		q.Set("timezone", timezone)
	}
	path := fmt.Sprintf("/api/chats/%s/messages/by-date", url.PathEscape(chatID))
	return c.doAuth(ctx, "GET", c.buildURL(path, q), nil, true)
}

// GetChatStats returns statistics for a specific chat.
func (c *Client) GetChatStats(ctx context.Context, chatID string) ([]byte, error) {
	path := fmt.Sprintf("/api/chats/%s/stats", url.PathEscape(chatID))
	return c.doAuth(ctx, "GET", c.buildURL(path, nil), nil, true)
}

// GetTopics returns topics for a chat.
func (c *Client) GetTopics(ctx context.Context, chatID string) ([]byte, error) {
	path := fmt.Sprintf("/api/chats/%s/topics", url.PathEscape(chatID))
	return c.doAuth(ctx, "GET", c.buildURL(path, nil), nil, true)
}

// RefreshStats triggers a forced recalculation of global stats.
func (c *Client) RefreshStats(ctx context.Context) ([]byte, error) {
	return c.doAuth(ctx, "POST", c.buildURL("/api/stats/refresh", nil), nil, true)
}
