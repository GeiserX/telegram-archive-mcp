package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// testMux returns a mux that handles login and configurable API endpoints.
func testMux(routes map[string]http.HandlerFunc) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "viewer_auth", Value: "test"})
		w.WriteHeader(http.StatusOK)
	})
	for pattern, handler := range routes {
		mux.HandleFunc(pattern, handler)
	}
	return mux
}

// newTestClient creates an httptest server and client for tool handler tests.
func newTestClient(t *testing.T, routes map[string]http.HandlerFunc) (*httptest.Server, *client.Client) {
	t.Helper()
	ts := httptest.NewServer(testMux(routes))
	c := client.New(ts.URL, "admin", "secret")
	return ts, c
}

// makeToolRequest builds a CallToolRequest with the given arguments.
func makeToolRequest(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}

// resultText extracts the text from the first TextContent in a CallToolResult.
func resultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("result has no content")
	}
	tc, ok := mcp.AsTextContent(result.Content[0])
	if !ok {
		// Fallback: marshal and check
		b, _ := json.Marshal(result.Content[0])
		t.Fatalf("first content is not TextContent: %s", string(b))
	}
	return tc.Text
}

// --- ListChats ---

func TestNewListChats_ReturnsResultOnSuccess(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`[{"id":"c1","name":"Test"}]`))
		},
	})
	defer ts.Close()

	_, handler := NewListChats(c)
	req := makeToolRequest(map[string]any{"limit": float64(50)})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result.IsError {
		t.Fatalf("result is error: %s", resultText(t, result))
	}
	text := resultText(t, result)
	if !strings.Contains(text, `"id":"c1"`) {
		t.Errorf("unexpected text: %s", text)
	}
}

func TestNewListChats_DefaultsLimitTo100(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("limit") != "100" {
				t.Errorf("limit = %q, want 100", r.URL.Query().Get("limit"))
			}
			w.Write([]byte(`[]`))
		},
	})
	defer ts.Close()

	_, handler := NewListChats(c)
	req := makeToolRequest(map[string]any{})

	_, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
}

func TestNewListChats_CapsLimitAt500(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("limit") != "500" {
				t.Errorf("limit = %q, want 500 (capped)", r.URL.Query().Get("limit"))
			}
			w.Write([]byte(`[]`))
		},
	})
	defer ts.Close()

	_, handler := NewListChats(c)
	req := makeToolRequest(map[string]any{"limit": float64(9999)})

	_, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
}

func TestNewListChats_ToolHasCorrectName(t *testing.T) {
	c := client.New("http://fake", "", "")
	tool, _ := NewListChats(c)
	if tool.Name != "list_chats" {
		t.Errorf("tool name = %q, want %q", tool.Name, "list_chats")
	}
}

func TestNewListChats_ToolIsReadOnly(t *testing.T) {
	c := client.New("http://fake", "", "")
	tool, _ := NewListChats(c)
	if tool.Annotations.ReadOnlyHint == nil || !*tool.Annotations.ReadOnlyHint {
		t.Error("list_chats should have ReadOnlyHint=true")
	}
}

// --- ListFolders ---

func TestNewListFolders_ReturnsResultOnSuccess(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/folders": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`[{"id":1,"title":"Work"}]`))
		},
	})
	defer ts.Close()

	_, handler := NewListFolders(c)
	req := makeToolRequest(map[string]any{})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result.IsError {
		t.Fatalf("result is error: %s", resultText(t, result))
	}
	text := resultText(t, result)
	if !strings.Contains(text, "Work") {
		t.Errorf("unexpected text: %s", text)
	}
}

func TestNewListFolders_ToolHasCorrectName(t *testing.T) {
	c := client.New("http://fake", "", "")
	tool, _ := NewListFolders(c)
	if tool.Name != "list_folders" {
		t.Errorf("tool name = %q, want %q", tool.Name, "list_folders")
	}
}

func TestNewListFolders_ToolIsReadOnly(t *testing.T) {
	c := client.New("http://fake", "", "")
	tool, _ := NewListFolders(c)
	if tool.Annotations.ReadOnlyHint == nil || !*tool.Annotations.ReadOnlyHint {
		t.Error("list_folders should have ReadOnlyHint=true")
	}
}

// --- SearchMessages ---

func TestNewSearchMessages_ReturnsResultOnSuccess(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`[{"id":1,"text":"hello"}]`))
		},
	})
	defer ts.Close()

	_, handler := NewSearchMessages(c)
	req := makeToolRequest(map[string]any{
		"chat_id": "c1",
		"query":   "hello",
		"limit":   float64(10),
	})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result.IsError {
		t.Fatalf("result is error: %s", resultText(t, result))
	}
	text := resultText(t, result)
	if !strings.Contains(text, `[{"id":1,"text":"hello"}]`) {
		t.Errorf("unexpected text: %s", text)
	}
}

func TestNewSearchMessages_ReturnErrorWhenChatIDMissing(t *testing.T) {
	ts, c := newTestClient(t, nil)
	defer ts.Close()

	_, handler := NewSearchMessages(c)
	req := makeToolRequest(map[string]any{
		"query": "hello",
	})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError when chat_id missing")
	}
}

func TestNewSearchMessages_ReturnErrorWhenQueryMissing(t *testing.T) {
	ts, c := newTestClient(t, nil)
	defer ts.Close()

	_, handler := NewSearchMessages(c)
	req := makeToolRequest(map[string]any{
		"chat_id": "c1",
	})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError when query missing")
	}
}

func TestNewSearchMessages_DefaultsLimitTo20(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("limit") != "20" {
				t.Errorf("limit = %q, want 20", r.URL.Query().Get("limit"))
			}
			w.Write([]byte(`[]`))
		},
	})
	defer ts.Close()

	_, handler := NewSearchMessages(c)
	req := makeToolRequest(map[string]any{
		"chat_id": "c1",
		"query":   "test",
	})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected error: %s", resultText(t, result))
	}
}

func TestNewSearchMessages_CapsLimitAt200(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("limit") != "200" {
				t.Errorf("limit = %q, want 200 (capped)", r.URL.Query().Get("limit"))
			}
			w.Write([]byte(`[]`))
		},
	})
	defer ts.Close()

	_, handler := NewSearchMessages(c)
	req := makeToolRequest(map[string]any{
		"chat_id": "c1",
		"query":   "test",
		"limit":   float64(9999),
	})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected error: %s", resultText(t, result))
	}
}

func TestNewSearchMessages_ReturnsToolErrorOnAPIFailure(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("boom"))
		},
	})
	defer ts.Close()

	_, handler := NewSearchMessages(c)
	req := makeToolRequest(map[string]any{
		"chat_id": "c1",
		"query":   "test",
	})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError on API failure")
	}
}

func TestNewSearchMessages_ToolIsReadOnly(t *testing.T) {
	c := client.New("http://fake", "", "")
	tool, _ := NewSearchMessages(c)
	if tool.Annotations.ReadOnlyHint == nil || !*tool.Annotations.ReadOnlyHint {
		t.Error("search_messages should have ReadOnlyHint=true")
	}
}

// --- GetMessages ---

func TestNewGetMessages_ReturnsResultOnSuccess(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`[{"id":1}]`))
		},
	})
	defer ts.Close()

	_, handler := NewGetMessages(c)
	req := makeToolRequest(map[string]any{
		"chat_id": "c1",
		"limit":   float64(10),
		"offset":  float64(5),
	})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result.IsError {
		t.Fatalf("result is error: %s", resultText(t, result))
	}
	text := resultText(t, result)
	if !strings.Contains(text, `[{"id":1}]`) {
		t.Errorf("unexpected text: %s", text)
	}
}

func TestNewGetMessages_ReturnErrorWhenChatIDMissing(t *testing.T) {
	ts, c := newTestClient(t, nil)
	defer ts.Close()

	_, handler := NewGetMessages(c)
	req := makeToolRequest(map[string]any{})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError when chat_id missing")
	}
}

func TestNewGetMessages_DefaultsLimitTo50(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("limit") != "50" {
				t.Errorf("limit = %q, want 50", r.URL.Query().Get("limit"))
			}
			w.Write([]byte(`[]`))
		},
	})
	defer ts.Close()

	_, handler := NewGetMessages(c)
	req := makeToolRequest(map[string]any{"chat_id": "c1"})

	_, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
}

func TestNewGetMessages_CapsLimitAt500(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("limit") != "500" {
				t.Errorf("limit = %q, want 500 (capped)", r.URL.Query().Get("limit"))
			}
			w.Write([]byte(`[]`))
		},
	})
	defer ts.Close()

	_, handler := NewGetMessages(c)
	req := makeToolRequest(map[string]any{
		"chat_id": "c1",
		"limit":   float64(99999),
	})

	_, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
}

func TestNewGetMessages_CapsOffsetAt100000(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("offset") != "100000" {
				t.Errorf("offset = %q, want 100000 (capped)", r.URL.Query().Get("offset"))
			}
			w.Write([]byte(`[]`))
		},
	})
	defer ts.Close()

	_, handler := NewGetMessages(c)
	req := makeToolRequest(map[string]any{
		"chat_id": "c1",
		"offset":  float64(999999),
	})

	_, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
}

func TestNewGetMessages_ReturnsToolErrorOnAPIFailure(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
	})
	defer ts.Close()

	_, handler := NewGetMessages(c)
	req := makeToolRequest(map[string]any{"chat_id": "c1"})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError on API failure")
	}
}

func TestNewGetMessages_ToolIsReadOnly(t *testing.T) {
	c := client.New("http://fake", "", "")
	tool, _ := NewGetMessages(c)
	if tool.Annotations.ReadOnlyHint == nil || !*tool.Annotations.ReadOnlyHint {
		t.Error("get_messages should have ReadOnlyHint=true")
	}
}

// --- GetPinnedMessages ---

func TestNewGetPinnedMessages_ReturnsResultOnSuccess(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/pinned": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`[{"pinned":true}]`))
		},
	})
	defer ts.Close()

	_, handler := NewGetPinnedMessages(c)
	req := makeToolRequest(map[string]any{"chat_id": "c1"})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result.IsError {
		t.Fatalf("result is error: %s", resultText(t, result))
	}
	text := resultText(t, result)
	if !strings.Contains(text, `[{"pinned":true}]`) {
		t.Errorf("unexpected text: %s", text)
	}
}

func TestNewGetPinnedMessages_ReturnErrorWhenChatIDMissing(t *testing.T) {
	ts, c := newTestClient(t, nil)
	defer ts.Close()

	_, handler := NewGetPinnedMessages(c)
	req := makeToolRequest(map[string]any{})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError when chat_id missing")
	}
}

func TestNewGetPinnedMessages_ReturnsToolErrorOnAPIFailure(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/pinned": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		},
	})
	defer ts.Close()

	_, handler := NewGetPinnedMessages(c)
	req := makeToolRequest(map[string]any{"chat_id": "c1"})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError on API failure")
	}
}

// --- GetMessagesByDate ---

func TestNewGetMessagesByDate_ReturnErrorWhenChatIDMissing(t *testing.T) {
	ts, c := newTestClient(t, nil)
	defer ts.Close()

	_, handler := NewGetMessagesByDate(c)
	req := makeToolRequest(map[string]any{"date": "2025-01-15"})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError when chat_id missing")
	}
}

func TestNewGetMessagesByDate_ReturnErrorWhenDateMissing(t *testing.T) {
	ts, c := newTestClient(t, nil)
	defer ts.Close()

	_, handler := NewGetMessagesByDate(c)
	req := makeToolRequest(map[string]any{"chat_id": "c1"})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError when date missing")
	}
}

func TestNewGetMessagesByDate_ReturnErrorOnInvalidDateFormat(t *testing.T) {
	ts, c := newTestClient(t, nil)
	defer ts.Close()

	_, handler := NewGetMessagesByDate(c)
	req := makeToolRequest(map[string]any{
		"chat_id": "c1",
		"date":    "15/01/2025",
	})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError on invalid date format")
	}
	text := resultText(t, result)
	if !strings.Contains(text, "YYYY-MM-DD") {
		t.Errorf("error should mention expected format, got: %s", text)
	}
}

// --- GetChatStats ---

func TestNewGetChatStats_ReturnsResultOnSuccess(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/stats": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"messages":100}`))
		},
	})
	defer ts.Close()

	_, handler := NewGetChatStats(c)
	req := makeToolRequest(map[string]any{"chat_id": "c1"})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result.IsError {
		t.Fatalf("result is error: %s", resultText(t, result))
	}
	text := resultText(t, result)
	if !strings.Contains(text, `{"messages":100}`) {
		t.Errorf("unexpected text: %s", text)
	}
}

func TestNewGetChatStats_ReturnErrorWhenChatIDMissing(t *testing.T) {
	ts, c := newTestClient(t, nil)
	defer ts.Close()

	_, handler := NewGetChatStats(c)
	req := makeToolRequest(map[string]any{})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError when chat_id missing")
	}
}

func TestNewGetChatStats_ReturnsToolErrorOnAPIFailure(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/stats": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		},
	})
	defer ts.Close()

	_, handler := NewGetChatStats(c)
	req := makeToolRequest(map[string]any{"chat_id": "c1"})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError on API failure")
	}
}

// --- GetTopics ---

func TestNewGetTopics_ReturnsResultOnSuccess(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/topics": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`[{"topic":"General"}]`))
		},
	})
	defer ts.Close()

	_, handler := NewGetTopics(c)
	req := makeToolRequest(map[string]any{"chat_id": "c1"})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result.IsError {
		t.Fatalf("result is error: %s", resultText(t, result))
	}
	text := resultText(t, result)
	if !strings.Contains(text, "General") {
		t.Errorf("unexpected text: %s", text)
	}
}

func TestNewGetTopics_ReturnErrorWhenChatIDMissing(t *testing.T) {
	ts, c := newTestClient(t, nil)
	defer ts.Close()

	_, handler := NewGetTopics(c)
	req := makeToolRequest(map[string]any{})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError when chat_id missing")
	}
}

func TestNewGetTopics_ReturnsToolErrorOnAPIFailure(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/topics": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
	})
	defer ts.Close()

	_, handler := NewGetTopics(c)
	req := makeToolRequest(map[string]any{"chat_id": "c1"})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError on API failure")
	}
}

// --- RefreshStats ---

func TestNewRefreshStats_ReturnsResultOnSuccess(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/stats/refresh": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"ok":true}`))
		},
	})
	defer ts.Close()

	_, handler := NewRefreshStats(c)
	req := makeToolRequest(map[string]any{})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result.IsError {
		t.Fatalf("result is error: %s", resultText(t, result))
	}
	text := resultText(t, result)
	if !strings.Contains(text, `{"ok":true}`) {
		t.Errorf("unexpected text: %s", text)
	}
}

func TestNewRefreshStats_ReturnsToolErrorOnAPIFailure(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/stats/refresh": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("server down"))
		},
	})
	defer ts.Close()

	_, handler := NewRefreshStats(c)
	req := makeToolRequest(map[string]any{})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError on API failure")
	}
}

func TestNewRefreshStats_ToolIsNotDestructive(t *testing.T) {
	c := client.New("http://fake", "", "")
	tool, _ := NewRefreshStats(c)
	if tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint {
		t.Error("refresh_stats should have DestructiveHint=false")
	}
}

// --- Tool definition checks ---

func TestNewSearchMessages_ToolHasCorrectName(t *testing.T) {
	c := client.New("http://fake", "", "")
	tool, _ := NewSearchMessages(c)
	if tool.Name != "search_messages" {
		t.Errorf("tool name = %q, want %q", tool.Name, "search_messages")
	}
}

func TestNewGetMessages_ToolHasCorrectName(t *testing.T) {
	c := client.New("http://fake", "", "")
	tool, _ := NewGetMessages(c)
	if tool.Name != "get_messages" {
		t.Errorf("tool name = %q, want %q", tool.Name, "get_messages")
	}
}

func TestNewGetPinnedMessages_ToolHasCorrectName(t *testing.T) {
	c := client.New("http://fake", "", "")
	tool, _ := NewGetPinnedMessages(c)
	if tool.Name != "get_pinned_messages" {
		t.Errorf("tool name = %q, want %q", tool.Name, "get_pinned_messages")
	}
}

func TestNewGetMessagesByDate_ToolHasCorrectName(t *testing.T) {
	c := client.New("http://fake", "", "")
	tool, _ := NewGetMessagesByDate(c)
	if tool.Name != "get_messages_by_date" {
		t.Errorf("tool name = %q, want %q", tool.Name, "get_messages_by_date")
	}
}

func TestNewGetChatStats_ToolHasCorrectName(t *testing.T) {
	c := client.New("http://fake", "", "")
	tool, _ := NewGetChatStats(c)
	if tool.Name != "get_chat_stats" {
		t.Errorf("tool name = %q, want %q", tool.Name, "get_chat_stats")
	}
}

func TestNewGetTopics_ToolHasCorrectName(t *testing.T) {
	c := client.New("http://fake", "", "")
	tool, _ := NewGetTopics(c)
	if tool.Name != "get_topics" {
		t.Errorf("tool name = %q, want %q", tool.Name, "get_topics")
	}
}

func TestNewRefreshStats_ToolHasCorrectName(t *testing.T) {
	c := client.New("http://fake", "", "")
	tool, _ := NewRefreshStats(c)
	if tool.Name != "refresh_stats" {
		t.Errorf("tool name = %q, want %q", tool.Name, "refresh_stats")
	}
}

func TestNewGetMessages_UsesKeysetCursorAndIgnoresOffset(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			if q.Get("before_date") != "2026-06-10T18:04:17" {
				t.Errorf("before_date = %q", q.Get("before_date"))
			}
			if q.Get("before_id") != "1142046" {
				t.Errorf("before_id = %q", q.Get("before_id"))
			}
			if q.Get("offset") != "" {
				t.Errorf("offset should be ignored with a cursor, got %q", q.Get("offset"))
			}
			if q.Get("limit") != "500" {
				t.Errorf("limit = %q", q.Get("limit"))
			}
			w.Write([]byte(`[{"id":1142045}]`))
		},
	})
	defer ts.Close()

	_, handler := NewGetMessages(c)
	req := makeToolRequest(map[string]any{
		"chat_id":     "c1",
		"limit":       float64(9000),
		"offset":      float64(10),
		"before_date": "2026-06-10T20:04:17+02:00",
		"before_id":   float64(1142046),
	})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result.IsError {
		t.Fatalf("result is error: %s", resultText(t, result))
	}
	if !strings.Contains(resultText(t, result), "1142045") {
		t.Errorf("unexpected text: %s", resultText(t, result))
	}
}

func TestNewGetMessages_AfterIDCursor(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			if q.Get("after_id") != "42" {
				t.Errorf("after_id = %q", q.Get("after_id"))
			}
			if q.Get("before_date") != "" || q.Get("before_id") != "" || q.Get("offset") != "" {
				t.Errorf("unexpected params: %s", r.URL.RawQuery)
			}
			w.Write([]byte(`[]`))
		},
	})
	defer ts.Close()

	_, handler := NewGetMessages(c)
	result, err := handler(context.Background(), makeToolRequest(map[string]any{
		"chat_id":  "c1",
		"after_id": float64(42),
	}))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result.IsError {
		t.Fatalf("result is error: %s", resultText(t, result))
	}
}

func TestNewGetMessages_RejectsMalformedBeforeDate(t *testing.T) {
	ts, c := newTestClient(t, nil)
	defer ts.Close()

	_, handler := NewGetMessages(c)
	result, err := handler(context.Background(), makeToolRequest(map[string]any{
		"chat_id":     "c1",
		"before_date": "10/06/2026",
	}))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !result.IsError || !strings.Contains(resultText(t, result), "invalid before_date") {
		t.Errorf("expected an invalid before_date error, got %s", resultText(t, result))
	}
}

func TestNewGetMessages_CursorReturnsToolErrorOnAPIFailure(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
	})
	defer ts.Close()

	_, handler := NewGetMessages(c)
	result, err := handler(context.Background(), makeToolRequest(map[string]any{
		"chat_id":   "c1",
		"before_id": float64(7),
	}))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !result.IsError {
		t.Error("expected a tool error")
	}
}

func TestNewGetMessages_RejectsFractionalCursorID(t *testing.T) {
	ts, c := newTestClient(t, nil)
	defer ts.Close()

	_, handler := NewGetMessages(c)
	result, err := handler(context.Background(), makeToolRequest(map[string]any{
		"chat_id":   "c1",
		"before_id": float64(42.5),
	}))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !result.IsError || !strings.Contains(resultText(t, result), "whole number") {
		t.Errorf("expected a whole-number error, got %s", resultText(t, result))
	}
}

func TestNewGetMessages_RejectsOutOfRangeAfterID(t *testing.T) {
	ts, c := newTestClient(t, nil)
	defer ts.Close()

	_, handler := NewGetMessages(c)
	result, err := handler(context.Background(), makeToolRequest(map[string]any{
		"chat_id":  "c1",
		"after_id": float64(1e30),
	}))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !result.IsError || !strings.Contains(resultText(t, result), "between 0 and") {
		t.Errorf("expected a range error, got %s", resultText(t, result))
	}
}

func TestNewGetMessages_RejectsNonNumericLimit(t *testing.T) {
	ts, c := newTestClient(t, nil)
	defer ts.Close()

	_, handler := NewGetMessages(c)
	result, err := handler(context.Background(), makeToolRequest(map[string]any{
		"chat_id": "c1",
		"limit":   "ten",
	}))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !result.IsError || !strings.Contains(resultText(t, result), "must be a number") {
		t.Errorf("expected a number error, got %s", resultText(t, result))
	}
}

func TestNewGetMessages_RejectsFractionalOffset(t *testing.T) {
	ts, c := newTestClient(t, nil)
	defer ts.Close()

	_, handler := NewGetMessages(c)
	result, err := handler(context.Background(), makeToolRequest(map[string]any{
		"chat_id": "c1",
		"offset":  float64(1.5),
	}))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !result.IsError || !strings.Contains(resultText(t, result), "whole number") {
		t.Errorf("expected a whole-number error, got %s", resultText(t, result))
	}
}
