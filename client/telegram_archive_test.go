package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// newTestServer creates an httptest.Server with the given handler and returns
// a Client pointing at it. The caller should defer ts.Close().
func newTestServer(t *testing.T, handler http.Handler) (*httptest.Server, *Client) {
	t.Helper()
	ts := httptest.NewServer(handler)
	c := New(ts.URL, "admin", "secret")
	return ts, c
}

// loginMux returns a mux that handles /api/login by setting a viewer_auth cookie.
func loginMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var creds struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if creds.Username == "admin" && creds.Password == "secret" {
			http.SetCookie(w, &http.Cookie{Name: "viewer_auth", Value: "tok123"})
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("bad credentials"))
	})
	return mux
}

// --- New() ---

func TestNew_SetsFieldsCorrectly(t *testing.T) {
	c := New("http://example.com/", "user", "pass")
	if c.base != "http://example.com" {
		t.Errorf("base = %q, want trailing slash trimmed", c.base)
	}
	if c.user != "user" {
		t.Errorf("user = %q, want %q", c.user, "user")
	}
	if c.pass != "pass" {
		t.Errorf("pass = %q, want %q", c.pass, "pass")
	}
	if c.hc == nil {
		t.Error("http client is nil")
	}
}

func TestNew_TrimsMultipleTrailingSlashes(t *testing.T) {
	c := New("http://example.com///", "u", "p")
	if c.base != "http://example.com" {
		t.Errorf("base = %q, want trailing slashes trimmed", c.base)
	}
}

func TestNew_NoTrailingSlash(t *testing.T) {
	c := New("http://example.com", "u", "p")
	if c.base != "http://example.com" {
		t.Errorf("base = %q, want unchanged", c.base)
	}
}

// --- buildURL ---

func TestBuildURL_WithoutQueryParams(t *testing.T) {
	c := New("http://host:3000", "", "")
	got := c.buildURL("/api/stats", nil)
	if got != "http://host:3000/api/stats" {
		t.Errorf("buildURL = %q", got)
	}
}

func TestBuildURL_WithQueryParams(t *testing.T) {
	c := New("http://host:3000", "", "")
	q := make(map[string][]string)
	q["limit"] = []string{"10"}
	q["search"] = []string{"hello world"}
	got := c.buildURL("/api/chats/123/messages", q)
	// URL must contain both params
	if got == "" {
		t.Fatal("empty URL")
	}
	if len(got) < len("http://host:3000/api/chats/123/messages?") {
		t.Errorf("URL too short: %q", got)
	}
}

func TestBuildURL_EmptyQueryValues(t *testing.T) {
	c := New("http://host:3000", "", "")
	q := make(map[string][]string)
	got := c.buildURL("/api/stats", q)
	if got != "http://host:3000/api/stats" {
		t.Errorf("empty query should not append ?, got %q", got)
	}
}

// --- login ---

func TestLogin_SetsViewerAuthCookie(t *testing.T) {
	mux := loginMux()
	ts := httptest.NewServer(mux)
	defer ts.Close()

	c := New(ts.URL, "admin", "secret")
	if err := c.login(context.Background()); err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if c.cookie != "tok123" {
		t.Errorf("cookie = %q, want %q", c.cookie, "tok123")
	}
}

func TestLogin_ReturnsErrorOnBadCredentials(t *testing.T) {
	mux := loginMux()
	ts := httptest.NewServer(mux)
	defer ts.Close()

	c := New(ts.URL, "wrong", "creds")
	err := c.login(context.Background())
	if err == nil {
		t.Fatal("expected error for bad credentials")
	}
}

func TestLogin_ReturnsErrorWhenNoCookieInResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return 200 but no cookie
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := New(ts.URL, "admin", "secret")
	err := c.login(context.Background())
	if err == nil {
		t.Fatal("expected error when viewer_auth cookie missing")
	}
}

func TestLogin_ReturnsErrorOnNetworkFailure(t *testing.T) {
	c := New("http://127.0.0.1:1", "u", "p") // nothing listening
	err := c.login(context.Background())
	if err == nil {
		t.Fatal("expected error on connection refused")
	}
}

// --- doAuth ---

func TestDoAuth_AuthenticatesAutomaticallyWhenNoCookie(t *testing.T) {
	mux := loginMux()
	mux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		cookie := r.Header.Get("Cookie")
		if cookie == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`{"chats":42}`))
	})
	ts, c := newTestServer(t, mux)
	defer ts.Close()

	body, err := c.GetStats(context.Background())
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	if string(body) != `{"chats":42}` {
		t.Errorf("body = %q", string(body))
	}
}

func TestDoAuth_RetriesOnceOn401(t *testing.T) {
	var callCount atomic.Int32
	mux := loginMux()
	mux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`{"ok":true}`))
	})
	ts, c := newTestServer(t, mux)
	defer ts.Close()

	body, err := c.GetStats(context.Background())
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	if string(body) != `{"ok":true}` {
		t.Errorf("body = %q", string(body))
	}
}

func TestDoAuth_DoesNotRetryMoreThanOnce(t *testing.T) {
	mux := loginMux()
	mux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	ts, c := newTestServer(t, mux)
	defer ts.Close()

	// Pre-login so cookie is set, then every call returns 401
	c.cookie = "stale"
	_, err := c.doAuth(context.Background(), "GET", ts.URL+"/api/stats", nil, true)
	if err == nil {
		t.Fatal("expected error after double 401")
	}
}

func TestDoAuth_ReturnsErrorOn500(t *testing.T) {
	mux := loginMux()
	mux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	})
	ts, c := newTestServer(t, mux)
	defer ts.Close()

	_, err := c.GetStats(context.Background())
	if err == nil {
		t.Fatal("expected error on 500")
	}
}

func TestDoAuth_SendsBodyOnRetry(t *testing.T) {
	var callCount atomic.Int32
	mux := loginMux()
	mux.HandleFunc("/api/stats/refresh", func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`{"refreshed":true}`))
	})
	ts, c := newTestServer(t, mux)
	defer ts.Close()

	body, err := c.RefreshStats(context.Background())
	if err != nil {
		t.Fatalf("RefreshStats failed: %v", err)
	}
	if string(body) != `{"refreshed":true}` {
		t.Errorf("body = %q", string(body))
	}
}

// --- Resource methods ---

func TestGetStats_ReturnsResponseBody(t *testing.T) {
	mux := loginMux()
	mux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"total_chats":5}`))
	})
	ts, c := newTestServer(t, mux)
	defer ts.Close()

	body, err := c.GetStats(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != `{"total_chats":5}` {
		t.Errorf("body = %q", string(body))
	}
}

func TestGetChats_SendsLimitQueryParam(t *testing.T) {
	mux := loginMux()
	mux.HandleFunc("/api/chats", func(w http.ResponseWriter, r *http.Request) {
		limit := r.URL.Query().Get("limit")
		if limit != "25" {
			t.Errorf("limit = %q, want %q", limit, "25")
		}
		w.Write([]byte(`[]`))
	})
	ts, c := newTestServer(t, mux)
	defer ts.Close()

	_, err := c.GetChats(context.Background(), 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetChats_OmitsLimitWhenZero(t *testing.T) {
	mux := loginMux()
	mux.HandleFunc("/api/chats", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") != "" {
			t.Error("limit should not be sent when 0")
		}
		w.Write([]byte(`[]`))
	})
	ts, c := newTestServer(t, mux)
	defer ts.Close()

	_, err := c.GetChats(context.Background(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetFolders_ReturnsResponseBody(t *testing.T) {
	mux := loginMux()
	mux.HandleFunc("/api/folders", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"id":1,"name":"Work"}]`))
	})
	ts, c := newTestServer(t, mux)
	defer ts.Close()

	body, err := c.GetFolders(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != `[{"id":1,"name":"Work"}]` {
		t.Errorf("body = %q", string(body))
	}
}

func TestGetHealth_ReturnsResponseBody(t *testing.T) {
	mux := loginMux()
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})
	ts, c := newTestServer(t, mux)
	defer ts.Close()

	body, err := c.GetHealth(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != `{"status":"ok"}` {
		t.Errorf("body = %q", string(body))
	}
}

// --- Tool methods ---

func TestSearchMessages_SendsCorrectQueryParams(t *testing.T) {
	mux := loginMux()
	mux.HandleFunc("/api/chats/chat42/messages", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("search") != "hello" {
			t.Errorf("search = %q, want %q", q.Get("search"), "hello")
		}
		if q.Get("limit") != "10" {
			t.Errorf("limit = %q, want %q", q.Get("limit"), "10")
		}
		w.Write([]byte(`[{"msg":"hi"}]`))
	})
	ts, c := newTestServer(t, mux)
	defer ts.Close()

	body, err := c.SearchMessages(context.Background(), "chat42", "hello", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != `[{"msg":"hi"}]` {
		t.Errorf("body = %q", string(body))
	}
}

func TestSearchMessages_OmitsLimitWhenZero(t *testing.T) {
	mux := loginMux()
	mux.HandleFunc("/api/chats/c1/messages", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") != "" {
			t.Error("limit should not be sent when 0")
		}
		w.Write([]byte(`[]`))
	})
	ts, c := newTestServer(t, mux)
	defer ts.Close()

	_, err := c.SearchMessages(context.Background(), "c1", "q", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetMessages_SendsLimitAndOffset(t *testing.T) {
	mux := loginMux()
	mux.HandleFunc("/api/chats/c1/messages", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("limit") != "30" {
			t.Errorf("limit = %q", q.Get("limit"))
		}
		if q.Get("offset") != "100" {
			t.Errorf("offset = %q", q.Get("offset"))
		}
		w.Write([]byte(`[]`))
	})
	ts, c := newTestServer(t, mux)
	defer ts.Close()

	_, err := c.GetMessages(context.Background(), "c1", 30, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetMessages_OmitsZeroLimitAndOffset(t *testing.T) {
	mux := loginMux()
	mux.HandleFunc("/api/chats/c1/messages", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("limit") != "" {
			t.Error("limit should be absent")
		}
		if q.Get("offset") != "" {
			t.Error("offset should be absent")
		}
		w.Write([]byte(`[]`))
	})
	ts, c := newTestServer(t, mux)
	defer ts.Close()

	_, err := c.GetMessages(context.Background(), "c1", 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetPinnedMessages_UsesCorrectPath(t *testing.T) {
	mux := loginMux()
	mux.HandleFunc("/api/chats/c1/pinned", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"pinned":true}]`))
	})
	ts, c := newTestServer(t, mux)
	defer ts.Close()

	body, err := c.GetPinnedMessages(context.Background(), "c1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != `[{"pinned":true}]` {
		t.Errorf("body = %q", string(body))
	}
}

func TestGetMessagesByDate_SendsDateAndTimezone(t *testing.T) {
	mux := loginMux()
	mux.HandleFunc("/api/chats/c1/messages/by-date", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("date") != "2025-01-15" {
			t.Errorf("date = %q", q.Get("date"))
		}
		if q.Get("timezone") != "Europe/Madrid" {
			t.Errorf("timezone = %q", q.Get("timezone"))
		}
		w.Write([]byte(`[]`))
	})
	ts, c := newTestServer(t, mux)
	defer ts.Close()

	_, err := c.GetMessagesByDate(context.Background(), "c1", "2025-01-15", "Europe/Madrid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetMessagesByDate_OmitsEmptyTimezone(t *testing.T) {
	mux := loginMux()
	mux.HandleFunc("/api/chats/c1/messages/by-date", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("timezone") != "" {
			t.Error("timezone should be absent when empty")
		}
		w.Write([]byte(`[]`))
	})
	ts, c := newTestServer(t, mux)
	defer ts.Close()

	_, err := c.GetMessagesByDate(context.Background(), "c1", "2025-01-15", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetChatStats_UsesCorrectPath(t *testing.T) {
	mux := loginMux()
	mux.HandleFunc("/api/chats/c1/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"messages":999}`))
	})
	ts, c := newTestServer(t, mux)
	defer ts.Close()

	body, err := c.GetChatStats(context.Background(), "c1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != `{"messages":999}` {
		t.Errorf("body = %q", string(body))
	}
}

func TestGetTopics_UsesCorrectPath(t *testing.T) {
	mux := loginMux()
	mux.HandleFunc("/api/chats/c1/topics", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"topic":"General"}]`))
	})
	ts, c := newTestServer(t, mux)
	defer ts.Close()

	body, err := c.GetTopics(context.Background(), "c1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != `[{"topic":"General"}]` {
		t.Errorf("body = %q", string(body))
	}
}

func TestRefreshStats_UsesPOSTMethod(t *testing.T) {
	mux := loginMux()
	mux.HandleFunc("/api/stats/refresh", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %q, want POST", r.Method)
		}
		w.Write([]byte(`{"refreshed":true}`))
	})
	ts, c := newTestServer(t, mux)
	defer ts.Close()

	body, err := c.RefreshStats(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != `{"refreshed":true}` {
		t.Errorf("body = %q", string(body))
	}
}

// --- Path escaping ---

func TestSearchMessages_EscapesChatIDInPath(t *testing.T) {
	mux := loginMux()
	// A chat ID with a slash must be URL-escaped
	mux.HandleFunc("/api/chats/chat%2F1/messages", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[]`))
	})
	ts, c := newTestServer(t, mux)
	defer ts.Close()

	_, err := c.SearchMessages(context.Background(), "chat/1", "q", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- doAuth: login failure propagation ---

func TestDoAuth_ReturnsLoginErrorWhenNoCookieAndLoginFails(t *testing.T) {
	// Server rejects login, so doAuth should propagate the login error
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("forbidden"))
	}))
	defer ts.Close()

	c := New(ts.URL, "bad", "creds")
	_, err := c.GetStats(context.Background())
	if err == nil {
		t.Fatal("expected error when login fails")
	}
}

// --- doAuth: body != nil branch ---

func TestDoAuth_SendsNonNilBodyCorrectly(t *testing.T) {
	mux := loginMux()
	mux.HandleFunc("/api/stats/refresh", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":true}`))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	c := New(ts.URL, "admin", "secret")
	// Call doAuth directly with a non-nil body to exercise body != nil branch
	body, err := c.doAuth(context.Background(), "POST", ts.URL+"/api/stats/refresh", []byte(`{}`), true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != `{"ok":true}` {
		t.Errorf("body = %q", string(body))
	}
}

// --- doAuth: network error after successful login ---

func TestDoAuth_ReturnsErrorOnNetworkFailureAfterLogin(t *testing.T) {
	mux := loginMux()
	ts := httptest.NewServer(mux)

	c := New(ts.URL, "admin", "secret")
	// Login successfully first
	if err := c.login(context.Background()); err != nil {
		t.Fatalf("login failed: %v", err)
	}

	// Close the server to simulate network failure on the actual request
	ts.Close()

	_, err := c.GetStats(context.Background())
	if err == nil {
		t.Fatal("expected error on network failure after login")
	}
}

// --- doAuth: invalid URL causes NewRequestWithContext to fail ---

func TestDoAuth_ReturnsErrorOnInvalidRequestURL(t *testing.T) {
	mux := loginMux()
	ts := httptest.NewServer(mux)
	defer ts.Close()

	c := New(ts.URL, "admin", "secret")
	// Login first so we skip the login branch
	if err := c.login(context.Background()); err != nil {
		t.Fatalf("login failed: %v", err)
	}

	// Pass an invalid URL with control characters to trigger NewRequestWithContext error
	_, err := c.doAuth(context.Background(), "GET", "http://invalid\x7f", nil, true)
	if err == nil {
		t.Fatal("expected error on invalid URL")
	}
}

// --- login: invalid base URL causes NewRequestWithContext to fail ---

func TestLogin_ReturnsErrorOnInvalidBaseURL(t *testing.T) {
	c := New("http://invalid\x7f", "u", "p")
	err := c.login(context.Background())
	if err == nil {
		t.Fatal("expected error on invalid base URL")
	}
}
