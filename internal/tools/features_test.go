package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestNewGetMessageContext_OrdersOldestFirstAroundTheMessage(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			switch {
			case q.Get("before_id") != "":
				if q.Get("before_id") != "11" || q.Get("limit") != "3" {
					t.Errorf("older half: before_id=%s limit=%s, want 11 and 3", q.Get("before_id"), q.Get("limit"))
				}
				w.Write([]byte(`[{"id":10},{"id":9},{"id":8}]`)) // newest first
			case q.Get("after_id") != "":
				if q.Get("after_id") != "10" || q.Get("limit") != "2" {
					t.Errorf("newer half: after_id=%s limit=%s, want 10 and 2", q.Get("after_id"), q.Get("limit"))
				}
				w.Write([]byte(`[{"id":12},{"id":11}]`)) // newest first
			default:
				t.Errorf("unexpected query %s", r.URL.RawQuery)
			}
		},
	})
	defer ts.Close()

	_, handler := NewGetMessageContext(c)
	result, err := handler(context.Background(), makeToolRequest(map[string]any{
		"chat_id": "c1", "message_id": float64(10), "before": float64(2), "after": float64(2),
	}))
	if err != nil || result.IsError {
		t.Fatalf("handler: %v %s", err, resultText(t, result))
	}
	var got struct {
		MessageID int64 `json:"message_id"`
		Before    int   `json:"before"`
		After     int   `json:"after"`
		Messages  []struct {
			ID int64 `json:"id"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(resultText(t, result)), &got); err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for _, m := range got.Messages {
		ids = append(ids, m.ID)
	}
	if want := []int64{8, 9, 10, 11, 12}; !equalIDs(ids, want) || got.Before != 2 || got.After != 2 || got.MessageID != 10 {
		t.Errorf("got ids %v before %d after %d, want %v before 2 after 2", ids, got.Before, got.After, want)
	}
	if result.StructuredContent == nil {
		t.Error("no structuredContent")
	}
}

func equalIDs(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestNewGetMessageContext_MissingMessageIsAnError(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`[{"id":9},{"id":8}]`)) // 10 was never archived
		},
	})
	defer ts.Close()
	_, handler := NewGetMessageContext(c)
	result, _ := handler(context.Background(), makeToolRequest(map[string]any{"chat_id": "c1", "message_id": float64(10)}))
	if !result.IsError || !strings.Contains(resultText(t, result), "not found") {
		t.Errorf("want a not-found error, got %s", resultText(t, result))
	}
}

func TestTopicID_IsForwardedByEveryMessageTool(t *testing.T) {
	var seen []string
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": func(w http.ResponseWriter, r *http.Request) {
			seen = append(seen, r.URL.Query().Get("topic_id"))
			w.Write([]byte(`[]`))
		},
	})
	defer ts.Close()

	for name, call := range map[string]func() (*mcp.CallToolResult, error){
		"get_messages": func() (*mcp.CallToolResult, error) {
			_, h := NewGetMessages(c)
			return h(context.Background(), makeToolRequest(map[string]any{"chat_id": "c1", "topic_id": float64(7)}))
		},
		"search_messages": func() (*mcp.CallToolResult, error) {
			_, h := NewSearchMessages(c)
			return h(context.Background(), makeToolRequest(map[string]any{"chat_id": "c1", "query": "x", "topic_id": float64(7)}))
		},
		"get_messages_by_date": func() (*mcp.CallToolResult, error) {
			_, h := NewGetMessagesByDate(c)
			return h(context.Background(), makeToolRequest(map[string]any{"chat_id": "c1", "date": "2026-06-10", "topic_id": float64(7)}))
		},
	} {
		seen = nil
		res, err := call()
		if err != nil || res.IsError {
			t.Fatalf("%s: %v %s", name, err, resultText(t, res))
		}
		if len(seen) == 0 || seen[0] != "7" {
			t.Errorf("%s: topic_id sent = %v, want 7", name, seen)
		}
	}
}

// okRoutes answers the message and search routes successfully, so a refusal
// in these tests can only come from the argument check under test.
func okRoutes() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"/api/chats/c1/messages": func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`[]`)) },
		"/api/search/messages":   func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"results":[]}`)) },
	}
}

func TestTopicID_RejectsZero(t *testing.T) {
	ts, c := newTestClient(t, okRoutes())
	defer ts.Close()
	_, h := NewGetMessages(c)
	if res, _ := h(context.Background(), makeToolRequest(map[string]any{"chat_id": "c1", "topic_id": float64(1)})); res.IsError {
		t.Fatalf("positive control: topic_id 1 refused: %s", resultText(t, res))
	}
	res, _ := h(context.Background(), makeToolRequest(map[string]any{"chat_id": "c1", "topic_id": float64(0)}))
	if !res.IsError {
		t.Error("topic_id 0 was accepted")
	}
}

func TestNewListChats_ForwardsFilters(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats": func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			if q.Get("search") != "climb" || q.Get("archived") != "false" || q.Get("folder_id") != "3" {
				t.Errorf("query = %s, want search=climb archived=false folder_id=3", r.URL.RawQuery)
			}
			w.Write([]byte(`{"chats":[],"total":0,"limit":100,"offset":0,"has_more":false}`))
		},
	})
	defer ts.Close()
	_, h := NewListChats(c)
	res, err := h(context.Background(), makeToolRequest(map[string]any{"search": "climb", "archived": false, "folder_id": float64(3)}))
	if err != nil || res.IsError {
		t.Fatalf("%v %s", err, resultText(t, res))
	}
}

func TestNewListChats_OmitsUnsetFilters(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats": func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			for _, k := range []string{"search", "archived", "folder_id"} {
				if q.Has(k) {
					t.Errorf("%s sent without being asked: %s", k, r.URL.RawQuery)
				}
			}
			w.Write([]byte(`{"chats":[]}`))
		},
	})
	defer ts.Close()
	_, h := NewListChats(c)
	if res, _ := h(context.Background(), makeToolRequest(map[string]any{})); res.IsError {
		t.Fatal(resultText(t, res))
	}
}

func TestNewSearchAllChats_SendsQueryAndPaging(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/search/messages": func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			if q.Get("q") != "gym address" || q.Get("limit") != "5" || q.Get("offset") != "10" {
				t.Errorf("query = %s", r.URL.RawQuery)
			}
			w.Write([]byte(`{"query":"gym address","results":[{"id":1,"chat":{"ref":"c1"}}],"has_more":false}`))
		},
	})
	defer ts.Close()
	_, h := NewSearchAllChats(c)
	res, err := h(context.Background(), makeToolRequest(map[string]any{"query": "gym address", "limit": float64(5), "offset": float64(10)}))
	if err != nil || res.IsError {
		t.Fatalf("%v %s", err, resultText(t, res))
	}
	if !strings.Contains(resultText(t, res), `"ref":"c1"`) || res.StructuredContent == nil {
		t.Errorf("result = %s", resultText(t, res))
	}
}

func TestNewSearchAllChats_OldViewerGetsAnActionableError(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/search/messages": func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) },
	})
	defer ts.Close()
	_, h := NewSearchAllChats(c)
	res, _ := h(context.Background(), makeToolRequest(map[string]any{"query": "x"}))
	if !res.IsError || !strings.Contains(resultText(t, res), "search_messages") {
		t.Errorf("want a hint to use search_messages, got %s", resultText(t, res))
	}
}

func TestSearchTools_RejectBlankQuery(t *testing.T) {
	ts, c := newTestClient(t, okRoutes())
	defer ts.Close()
	_, one := NewSearchMessages(c)
	_, all := NewSearchAllChats(c)
	for name, res := range map[string]*mcp.CallToolResult{
		"positive control": must(one(context.Background(), makeToolRequest(map[string]any{"chat_id": "c1", "query": "x"}))),
	} {
		if res.IsError {
			t.Fatalf("%s: a real query was refused: %s", name, resultText(t, res))
		}
	}
	for name, res := range map[string]*mcp.CallToolResult{
		"search_messages":  must(one(context.Background(), makeToolRequest(map[string]any{"chat_id": "c1", "query": "  "}))),
		"search_all_chats": must(all(context.Background(), makeToolRequest(map[string]any{"query": ""}))),
	} {
		if !res.IsError {
			t.Errorf("%s accepted a blank query", name)
		}
	}
}

func must(r *mcp.CallToolResult, err error) *mcp.CallToolResult {
	if err != nil {
		panic(err)
	}
	return r
}

func TestJSONResult_WrapsArraysAndKeepsLargeIDsExact(t *testing.T) {
	body := `[{"id":9007199254740993}]`
	res := jsonResult([]byte(body), "messages")
	if res.IsError {
		t.Fatal(resultText(t, res))
	}
	if resultText(t, res) != body {
		t.Errorf("text changed: %s", resultText(t, res))
	}
	b, _ := json.Marshal(res.StructuredContent)
	if string(b) != `{"messages":[{"id":9007199254740993}]}` {
		t.Errorf("structuredContent = %s", b)
	}
	if res := jsonResult([]byte(`"just a string"`), "messages"); !res.IsError {
		t.Error("a JSON string was accepted as a result")
	}
	if res := jsonResult([]byte(`[1]`), ""); !res.IsError {
		t.Error("an array was accepted where an object is expected")
	}
}
