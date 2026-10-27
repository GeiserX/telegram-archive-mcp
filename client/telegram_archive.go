package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const maxResponseBody = 10 << 20 // 10 MB

// TooLargeError reports a response body larger than the caller's limit. The
// body is not returned.
type TooLargeError struct {
	Limit int64
}

func (e *TooLargeError) Error() string {
	return fmt.Sprintf("telegram-archive response exceeds %d bytes", e.Limit)
}

// StatusError is a viewer answer with status 400 or above. Its message is a
// fixed sentence chosen by the status, never the viewer's body: a reverse proxy
// or a debug-mode viewer could otherwise put stack traces or internal hostnames
// in front of the model.
type StatusError struct {
	Status int
	Login  bool // the error came from POST /api/login
}

func (e *StatusError) Error() string {
	if e.Login {
		switch e.Status {
		case http.StatusTooManyRequests:
			return "login failed (429): the archive allows 15 logins per client IP in 5 minutes; wait before retrying"
		case http.StatusUnauthorized, http.StatusForbidden:
			return fmt.Sprintf("login failed (%d): TELEGRAM_ARCHIVE_USER or TELEGRAM_ARCHIVE_PASS does not match a viewer login", e.Status)
		}
		return fmt.Sprintf("login failed (%d): the archive viewer did not accept the login", e.Status)
	}
	switch {
	case e.Status == http.StatusBadRequest || e.Status == http.StatusUnprocessableEntity:
		return fmt.Sprintf("telegram-archive error %d: the archive rejected the request arguments", e.Status)
	case e.Status == http.StatusUnauthorized:
		return "telegram-archive error 401: the archive refused the session; check TELEGRAM_ARCHIVE_USER and TELEGRAM_ARCHIVE_PASS"
	case e.Status == http.StatusForbidden:
		return "telegram-archive error 403: this archive login may not see that chat, file or action"
	case e.Status == http.StatusNotFound:
		return "telegram-archive error 404: not found; check chat_id (the ref from list_chats) and the message id"
	case e.Status == http.StatusTooManyRequests:
		return "telegram-archive error 429: the archive is rate limiting this server; wait before retrying"
	case e.Status >= 500:
		return fmt.Sprintf("telegram-archive error %d: the archive failed to answer; try again later", e.Status)
	}
	return fmt.Sprintf("telegram-archive error %d", e.Status)
}

// errCrossHostRedirect stops a redirect that would leave the viewer's host or
// drop from https to http. A 307 or 308 replays the request body, and the login
// body holds the password.
var errCrossHostRedirect = errors.New("the archive redirected to another host or to plain http")

func sameHostRedirects(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	first := via[0].URL
	if req.URL.Host != first.Host || (first.Scheme == "https" && req.URL.Scheme != "https") {
		return errCrossHostRedirect
	}
	return nil
}

// transportError turns a failed round trip into a sentence for the model. The
// raw error names the viewer's internal URL, so it goes to the server log only.
func transportError(op string, err error) error {
	log.Printf("telegram-archive %s failed: %v", op, err)
	switch {
	case errors.Is(err, errCrossHostRedirect):
		return fmt.Errorf("%s: %w; refusing to follow", op, errCrossHostRedirect)
	case errors.Is(err, context.Canceled):
		return fmt.Errorf("%s: request cancelled", op)
	case errors.Is(err, context.DeadlineExceeded) || isTimeout(err):
		return fmt.Errorf("%s: the archive viewer did not answer in time", op)
	}
	return fmt.Errorf("%s: cannot reach the archive viewer; check TELEGRAM_ARCHIVE_URL", op)
}

func isTimeout(err error) bool {
	var t interface{ Timeout() bool }
	return errors.As(err, &t) && t.Timeout()
}

// ErrInvalidRef rejects a chat ref or media key that is empty or a dot
// segment. url.PathEscape leaves "." and "..", so a ref of ".." would turn
// /api/chats/../stats into a different viewer route.
var ErrInvalidRef = errors.New("invalid id: it must not be empty, \".\" or \"..\"")

func escapeRef(ref string) (string, error) {
	if ref == "" || ref == "." || ref == ".." {
		return "", ErrInvalidRef
	}
	return url.PathEscape(ref), nil
}

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
		hc:   &http.Client{Timeout: 30 * time.Second, CheckRedirect: sameHostRedirects},
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
		log.Printf("telegram-archive login: build request: %v", err)
		return errors.New("login: TELEGRAM_ARCHIVE_URL is not a valid URL")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.hc.Do(req)
	if err != nil {
		return transportError("login", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return &StatusError{Status: resp.StatusCode, Login: true}
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
	return c.doAuthLimit(ctx, method, rawurl, body, retry, maxResponseBody)
}

// doAuthLimit is doAuth with a cap on the response body: a larger body
// returns a *TooLargeError.
func (c *Client) doAuthLimit(ctx context.Context, method, rawurl string, body []byte, retry bool, limit int64) ([]byte, error) {
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
		log.Printf("telegram-archive build request: %v", err)
		return nil, errors.New("build request: TELEGRAM_ARCHIVE_URL is not a valid URL")
	}
	req.AddCookie(&http.Cookie{Name: "viewer_auth", Value: cookie})

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, transportError(method+" request", err)
	}
	defer resp.Body.Close()

	// Re-authenticate on 401 (expired cookie), but only once. Clear the cookie
	// only if it is still the one this request sent: another call may already
	// have logged in again, and dropping its fresh cookie would cost a second
	// login against the viewer's per-IP login limit.
	if resp.StatusCode == 401 && retry {
		c.mu.Lock()
		if c.cookie == cookie {
			c.cookie = ""
		}
		c.mu.Unlock()

		return c.doAuthLimit(ctx, method, rawurl, body, false, limit)
	}

	if resp.StatusCode >= 400 {
		return nil, &StatusError{Status: resp.StatusCode}
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, transportError("reading the answer", err)
	}
	if int64(len(b)) > limit {
		return nil, &TooLargeError{Limit: limit}
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

// ChatsQuery holds the filters GET /api/chats accepts. Zero values are omitted.
type ChatsQuery struct {
	Limit  int
	Offset int
	// Search matches the chat title, first or last name, or username.
	Search string
	// Archived, when set, keeps only archived (true) or unarchived (false) chats.
	Archived *bool
	// FolderID, when set, keeps only the chats of one folder from GetFolders.
	FolderID *int64
}

// ListChats returns one page of chats, most recent first.
func (c *Client) ListChats(ctx context.Context, cq ChatsQuery) ([]byte, error) {
	q := url.Values{}
	if cq.Limit > 0 {
		q.Set("limit", strconv.Itoa(cq.Limit))
	}
	if cq.Offset > 0 {
		q.Set("offset", strconv.Itoa(cq.Offset))
	}
	if cq.Search != "" {
		q.Set("search", cq.Search)
	}
	if cq.Archived != nil {
		q.Set("archived", strconv.FormatBool(*cq.Archived))
	}
	if cq.FolderID != nil {
		q.Set("folder_id", strconv.FormatInt(*cq.FolderID, 10))
	}
	return c.doAuth(ctx, "GET", c.buildURL("/api/chats", q), nil, true)
}

// GetChats returns a list of chats.
func (c *Client) GetChats(ctx context.Context, limit, offset int) ([]byte, error) {
	return c.ListChats(ctx, ChatsQuery{Limit: limit, Offset: offset})
}

// SearchAllMessages searches message text across every chat the login may
// see (GET /api/search/messages). Each hit names its chat by ref.
func (c *Client) SearchAllMessages(ctx context.Context, query string, limit, offset int) ([]byte, error) {
	q := url.Values{}
	q.Set("q", query)
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		q.Set("offset", strconv.Itoa(offset))
	}
	return c.doAuth(ctx, "GET", c.buildURL("/api/search/messages", q), nil, true)
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

// MessagesCursor is the keyset cursor accepted by GET /api/chats/{ref}/messages.
// The viewer returns messages newest first. BeforeDate and BeforeID together
// page backwards in constant time (pass the date and id of the last message
// received); a lone BeforeID returns messages with a smaller id; AfterID
// returns the messages right after an id. Zero values are omitted from the
// query.
type MessagesCursor struct {
	BeforeDate string
	BeforeID   int64
	AfterID    int64
}

// MessagesQuery holds everything GET /api/chats/{ref}/messages accepts.
// Zero values are omitted from the query.
type MessagesQuery struct {
	Limit  int
	Offset int
	// Search keeps the messages whose text matches.
	Search string
	// TopicID keeps one forum topic (get_topics); 1 is the General topic.
	TopicID int64
	Cursor  MessagesCursor
}

// ListMessages returns messages of one chat, newest first.
func (c *Client) ListMessages(ctx context.Context, chatID string, mq MessagesQuery) ([]byte, error) {
	ref, err := escapeRef(chatID)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	if mq.Search != "" {
		q.Set("search", mq.Search)
	}
	if mq.Limit > 0 {
		q.Set("limit", strconv.Itoa(mq.Limit))
	}
	if mq.Offset > 0 {
		q.Set("offset", strconv.Itoa(mq.Offset))
	}
	if mq.Cursor.BeforeDate != "" {
		q.Set("before_date", mq.Cursor.BeforeDate)
	}
	if mq.Cursor.BeforeID > 0 {
		q.Set("before_id", strconv.FormatInt(mq.Cursor.BeforeID, 10))
	}
	if mq.Cursor.AfterID > 0 {
		q.Set("after_id", strconv.FormatInt(mq.Cursor.AfterID, 10))
	}
	if mq.TopicID > 0 {
		q.Set("topic_id", strconv.FormatInt(mq.TopicID, 10))
	}
	return c.doAuth(ctx, "GET", c.buildURL("/api/chats/"+ref+"/messages", q), nil, true)
}

// SearchMessages searches messages in a chat.
func (c *Client) SearchMessages(ctx context.Context, chatID, query string, limit int) ([]byte, error) {
	return c.ListMessages(ctx, chatID, MessagesQuery{Search: query, Limit: limit})
}

// GetMessages retrieves messages from a chat with pagination.
func (c *Client) GetMessages(ctx context.Context, chatID string, limit, offset int) ([]byte, error) {
	return c.ListMessages(ctx, chatID, MessagesQuery{Limit: limit, Offset: offset})
}

// GetMessagesCursor retrieves messages from a chat using the keyset cursor
// instead of an offset.
func (c *Client) GetMessagesCursor(ctx context.Context, chatID string, limit int, cur MessagesCursor) ([]byte, error) {
	return c.ListMessages(ctx, chatID, MessagesQuery{Limit: limit, Cursor: cur})
}

// GetPinnedMessages returns pinned messages for a chat.
func (c *Client) GetPinnedMessages(ctx context.Context, chatID string) ([]byte, error) {
	ref, err := escapeRef(chatID)
	if err != nil {
		return nil, err
	}
	path := "/api/chats/" + ref + "/pinned"
	return c.doAuth(ctx, "GET", c.buildURL(path, nil), nil, true)
}

// GetMessagesByDate retrieves messages from a specific date.
func (c *Client) GetMessagesByDate(ctx context.Context, chatID, date, timezone string) ([]byte, error) {
	q := url.Values{}
	q.Set("date", date)
	if timezone != "" {
		q.Set("timezone", timezone)
	}
	ref, err := escapeRef(chatID)
	if err != nil {
		return nil, err
	}
	path := "/api/chats/" + ref + "/messages/by-date"
	return c.doAuth(ctx, "GET", c.buildURL(path, q), nil, true)
}

// GetChatStats returns statistics for a specific chat.
func (c *Client) GetChatStats(ctx context.Context, chatID string) ([]byte, error) {
	ref, err := escapeRef(chatID)
	if err != nil {
		return nil, err
	}
	path := "/api/chats/" + ref + "/stats"
	return c.doAuth(ctx, "GET", c.buildURL(path, nil), nil, true)
}

// GetTopics returns topics for a chat.
func (c *Client) GetTopics(ctx context.Context, chatID string) ([]byte, error) {
	ref, err := escapeRef(chatID)
	if err != nil {
		return nil, err
	}
	path := "/api/chats/" + ref + "/topics"
	return c.doAuth(ctx, "GET", c.buildURL(path, nil), nil, true)
}

// RefreshStats triggers a forced recalculation of global stats.
func (c *Client) RefreshStats(ctx context.Context) ([]byte, error) {
	return c.doAuth(ctx, "POST", c.buildURL("/api/stats/refresh", nil), nil, true)
}

// GetMedia downloads the file of one media item, addressed by chat ref and
// media key ({message_id}_{type}, the media.id of a message). A file larger
// than maxBytes returns a *TooLargeError.
func (c *Client) GetMedia(ctx context.Context, chatID, mediaKey string, maxBytes int64) ([]byte, error) {
	ref, err := escapeRef(chatID)
	if err != nil {
		return nil, err
	}
	key, err := escapeRef(mediaKey)
	if err != nil {
		return nil, err
	}
	path := "/media/" + ref + "/" + key
	return c.doAuthLimit(ctx, "GET", c.buildURL(path, nil), nil, true, maxBytes)
}
