package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// fakeViewer answers every route the tools call with the shapes the
// Telegram Archive viewer returns.
func fakeViewer(t *testing.T) *httptest.Server {
	t.Helper()
	msg := func(id int) map[string]any {
		return map[string]any{"id": id, "date": "2026-06-10T12:00:00", "text": "hi", "reply_to_msg_id": nil,
			"media": map[string]any{"id": "10_document", "type": "document", "file_name": "a.pdf", "mime_type": "application/pdf", "downloaded": true, "url": "/media/c1/10_document"}}
	}
	write := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "viewer_auth", Value: "tok"})
	})
	mux.HandleFunc("/api/chats", func(w http.ResponseWriter, r *http.Request) {
		write(w, map[string]any{"chats": []any{map[string]any{"ref": "c1", "id": -100123, "type": "group", "title": "G"}}, "total": 1, "limit": 100, "offset": 0, "has_more": false})
	})
	mux.HandleFunc("/api/folders", func(w http.ResponseWriter, r *http.Request) {
		write(w, map[string]any{"folders": []any{map[string]any{"id": 2, "title": "Work"}}})
	})
	mux.HandleFunc("/api/search/messages", func(w http.ResponseWriter, r *http.Request) {
		write(w, map[string]any{"query": "hi", "limit": 20, "offset": 0, "has_more": false, "indexed": true,
			"results": []any{map[string]any{"id": 10, "text": "hi", "chat": map[string]any{"ref": "c1"}}}})
	})
	mux.HandleFunc("/api/chats/c1/messages", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("after_id") != "" {
			write(w, []any{msg(11)})
			return
		}
		write(w, []any{msg(10), msg(9)})
	})
	mux.HandleFunc("/api/chats/c1/pinned", func(w http.ResponseWriter, r *http.Request) { write(w, []any{msg(10)}) })
	mux.HandleFunc("/api/chats/c1/stats", func(w http.ResponseWriter, r *http.Request) {
		write(w, map[string]any{"messages": 2, "media_files": 1, "total_size_bytes": 10})
	})
	mux.HandleFunc("/api/chats/c1/topics", func(w http.ResponseWriter, r *http.Request) {
		write(w, map[string]any{"topics": []any{map[string]any{"id": 1, "title": "General"}}})
	})
	mux.HandleFunc("/api/stats/refresh", func(w http.ResponseWriter, r *http.Request) { write(w, map[string]any{"chats": 1}) })
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

// Every tool's structuredContent must match the outputSchema it declares:
// a client that validates would otherwise reject the answer.
func TestTools_StructuredOutputMatchesTheirSchema(t *testing.T) {
	ts := fakeViewer(t)
	s := newServer(client.New(ts.URL, "u", "p"), serverOptions{}, server.WithOutputSchemaValidation())
	calls := map[string]map[string]any{
		"list_chats":           {"search": "g", "archived": false, "folder_id": 2},
		"list_folders":         {},
		"search_all_chats":     {"query": "hi"},
		"search_messages":      {"chat_id": "c1", "query": "hi", "topic_id": 1},
		"get_messages":         {"chat_id": "c1", "limit": 2},
		"get_message_context":  {"chat_id": "c1", "message_id": 10, "before": 1, "after": 1},
		"get_pinned_messages":  {"chat_id": "c1"},
		"get_messages_by_date": {"chat_id": "c1", "date": "2026-06-10"},
		"get_chat_stats":       {"chat_id": "c1"},
		"get_topics":           {"chat_id": "c1"},
		"get_media":            {"chat_id": "c1", "message_id": 10},
		"refresh_stats":        {},
	}
	registered := s.ListTools()
	if len(registered) != len(calls) {
		t.Fatalf("server has %d tools, the test covers %d: add the new tool here", len(registered), len(calls))
	}
	for name, args := range calls {
		if registered[name] == nil {
			t.Fatalf("tool %s is not registered", name)
		}
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call",
			"params": map[string]any{"name": name, "arguments": args}})
		reply := s.HandleMessage(context.Background(), raw)
		resp, ok := reply.(mcp.JSONRPCResponse)
		if !ok {
			t.Errorf("%s: reply %T, want a response", name, reply)
			continue
		}
		res, ok := resp.Result.(*mcp.CallToolResult)
		if !ok {
			t.Errorf("%s: result %T", name, resp.Result)
			continue
		}
		if res.IsError {
			b, _ := json.Marshal(res.Content)
			t.Errorf("%s: isError: %s", name, b)
			continue
		}
		if res.StructuredContent == nil {
			t.Errorf("%s: no structuredContent", name)
		}
		if len(res.Content) == 0 {
			t.Errorf("%s: no text content for older clients", name)
		}
	}
}

func TestTools_EveryToolHasATitleAndAllFourHints(t *testing.T) {
	s := newServer(client.New("http://127.0.0.1:1", "", ""), serverOptions{})
	for name, st := range s.ListTools() {
		tool := st.Tool
		a := tool.Annotations
		if tool.Title == "" || a.Title == "" {
			t.Errorf("%s: missing title", name)
		}
		if a.ReadOnlyHint == nil || a.DestructiveHint == nil || a.IdempotentHint == nil || a.OpenWorldHint == nil {
			t.Errorf("%s: a behaviour hint is unset: %+v", name, a)
			continue
		}
		if *a.DestructiveHint || *a.OpenWorldHint {
			t.Errorf("%s: destructive=%v openWorld=%v, want both false", name, *a.DestructiveHint, *a.OpenWorldHint)
		}
		if want := name != "refresh_stats"; *a.ReadOnlyHint != want {
			t.Errorf("%s: readOnlyHint = %v, want %v", name, *a.ReadOnlyHint, want)
		}
		if len(tool.RawOutputSchema) == 0 {
			t.Errorf("%s: no outputSchema", name)
		}
	}
}

func TestReadOnly_HidesRefreshStats(t *testing.T) {
	c := client.New("http://127.0.0.1:1", "", "")
	full := s2names(newServer(c, serverOptions{}))
	ro := s2names(newServer(c, serverOptions{ReadOnly: true}))
	if !slices.Contains(full, "refresh_stats") {
		t.Fatalf("refresh_stats missing without READ_ONLY: %v", full)
	}
	if slices.Contains(ro, "refresh_stats") {
		t.Errorf("READ_ONLY still offers refresh_stats: %v", ro)
	}
	if len(ro) != len(full)-1 {
		t.Errorf("READ_ONLY hid %d tools, want exactly 1", len(full)-len(ro))
	}
}

func s2names(s *server.MCPServer) []string {
	var out []string
	for name := range s.ListTools() {
		out = append(out, name)
	}
	return out
}

func TestReadOnlyFromEnv(t *testing.T) {
	cases := map[string]struct {
		want    bool
		wantErr bool
	}{
		"": {false, false}, "1": {true, false}, "true": {true, false}, "TRUE": {true, false},
		"0": {false, false}, "false": {false, false}, "yes": {false, true}, "on": {false, true},
	}
	for v, tc := range cases {
		t.Setenv("READ_ONLY", v)
		got, err := readOnlyFromEnv()
		if (err != nil) != tc.wantErr || got != tc.want {
			t.Errorf("READ_ONLY=%q: got %v, %v; want %v, error %v", v, got, err, tc.want, tc.wantErr)
		}
	}
}

func TestTools_InputSchemaRejectsUnknownAndOutOfRangeArguments(t *testing.T) {
	ts := fakeViewer(t)
	s := newServer(client.New(ts.URL, "u", "p"), serverOptions{})
	call := func(args map[string]any) bool { // reports whether the call was refused
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call",
			"params": map[string]any{"name": "list_chats", "arguments": args}})
		resp, ok := s.HandleMessage(context.Background(), raw).(mcp.JSONRPCResponse)
		if !ok {
			return true // a protocol error is also a refusal
		}
		res, ok := resp.Result.(*mcp.CallToolResult)
		return !ok || res.IsError
	}
	if call(map[string]any{"limit": 500}) {
		t.Fatal("positive control: a valid list_chats call was refused")
	}
	for _, args := range []map[string]any{
		{"limit": 501},
		{"limit": 2.5},
		{"no_such_argument": true},
	} {
		if !call(args) {
			t.Errorf("list_chats %v was accepted", args)
		}
	}
}
