package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// byDateRow is the minimal message the day walker reads.
type byDateRow struct {
	ID   int64  `json:"id"`
	Date string `json:"date"`
	Text string `json:"text"`
}

func writeRows(w http.ResponseWriter, rows []byDateRow) {
	b, _ := json.Marshal(rows)
	w.Write(b)
}

func decodeByDate(t *testing.T, text string) byDateResult {
	t.Helper()
	var out byDateResult
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("result is not JSON: %v\n%s", err, text)
	}
	return out
}

func idsOf(t *testing.T, msgs []json.RawMessage) []int64 {
	t.Helper()
	ids := make([]int64, 0, len(msgs))
	for _, raw := range msgs {
		var m byDateRow
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("bad message: %v", err)
		}
		ids = append(ids, m.ID)
	}
	return ids
}

func TestNewGetMessagesByDate_ReturnsResultOnSuccess(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": func(w http.ResponseWriter, r *http.Request) {
			writeRows(w, []byDateRow{{ID: 20, Date: "2025-01-15T10:00:00", Text: "hello"}})
		},
	})
	defer ts.Close()

	_, handler := NewGetMessagesByDate(c)
	req := makeToolRequest(map[string]any{
		"chat_id":  "c1",
		"date":     "2025-01-15",
		"timezone": "Europe/Madrid",
	})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result.IsError {
		t.Fatalf("result is error: %s", resultText(t, result))
	}
	out := decodeByDate(t, resultText(t, result))
	if out.Date != "2025-01-15" || out.Timezone != "Europe/Madrid" || out.Count != 1 || out.Truncated {
		t.Errorf("unexpected envelope: %+v", out)
	}
	if !strings.Contains(string(out.Messages[0]), "hello") {
		t.Errorf("message body lost: %s", out.Messages[0])
	}
}

func TestNewGetMessagesByDate_WorksWithoutTimezone(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": func(w http.ResponseWriter, r *http.Request) {
			// UTC day: the first call must start at the next midnight UTC
			if r.URL.Query().Get("before_id") == "" {
				if got := r.URL.Query().Get("before_date"); got != "2025-01-16T00:00:00" {
					t.Errorf("before_date = %q", got)
				}
			}
			writeRows(w, []byDateRow{{ID: 20, Date: "2025-01-15T23:59:59"}})
		},
	})
	defer ts.Close()

	_, handler := NewGetMessagesByDate(c)
	req := makeToolRequest(map[string]any{
		"chat_id": "c1",
		"date":    "2025-01-15",
	})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result.IsError {
		t.Fatalf("result is error: %s", resultText(t, result))
	}
	out := decodeByDate(t, resultText(t, result))
	if out.Timezone != "UTC" || out.Count != 1 {
		t.Errorf("unexpected envelope: %+v", out)
	}
}

func TestNewGetMessagesByDate_KeepsOnlyThatDayOldestFirst(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": func(w http.ResponseWriter, r *http.Request) {
			writeRows(w, []byDateRow{
				{ID: 40, Date: "2025-01-16T00:30:00"}, // next day (defensive: the API should not return it)
				{ID: 30, Date: "2025-01-15T18:00:00"},
				{ID: 20, Date: "2025-01-15T09:00:00"},
				{ID: 10, Date: "2025-01-14T23:59:59"}, // previous day: stop here
				{ID: 5, Date: "2025-01-14T10:00:00"},
			})
		},
	})
	defer ts.Close()

	_, handler := NewGetMessagesByDate(c)
	result, err := handler(context.Background(), makeToolRequest(map[string]any{
		"chat_id": "c1",
		"date":    "2025-01-15",
	}))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result.IsError {
		t.Fatalf("result is error: %s", resultText(t, result))
	}
	out := decodeByDate(t, resultText(t, result))
	got := idsOf(t, out.Messages)
	if fmt.Sprint(got) != "[20 30]" {
		t.Errorf("ids = %v, want [20 30]", got)
	}
	if out.Count != 2 || out.Truncated {
		t.Errorf("unexpected envelope: %+v", out)
	}
}

func TestNewGetMessagesByDate_PagesWithCursorUntilDayStart(t *testing.T) {
	var calls []string
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			calls = append(calls, q.Get("before_date")+"|"+q.Get("before_id"))
			if q.Get("limit") != "500" {
				t.Errorf("page size = %q", q.Get("limit"))
			}
			switch q.Get("before_id") {
			case "":
				writeRows(w, []byDateRow{
					{ID: 20, Date: "2025-01-15T20:00:00"},
					{ID: 19, Date: "2025-01-15T19:00:00"},
				})
			case "19":
				writeRows(w, []byDateRow{
					{ID: 18, Date: "2025-01-15T08:00:00"},
					{ID: 7, Date: "2025-01-14T22:00:00"},
				})
			default:
				t.Errorf("unexpected cursor %q", q.Get("before_id"))
				writeRows(w, nil)
			}
		},
	})
	defer ts.Close()

	_, handler := NewGetMessagesByDate(c)
	result, err := handler(context.Background(), makeToolRequest(map[string]any{
		"chat_id": "c1",
		"date":    "2025-01-15",
	}))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result.IsError {
		t.Fatalf("result is error: %s", resultText(t, result))
	}
	out := decodeByDate(t, resultText(t, result))
	if got := idsOf(t, out.Messages); fmt.Sprint(got) != "[18 19 20]" {
		t.Errorf("ids = %v, want [18 19 20]", got)
	}
	if len(calls) != 2 || calls[0] != "2025-01-16T00:00:00|" || calls[1] != "2025-01-15T19:00:00|19" {
		t.Errorf("cursor calls = %v", calls)
	}
}

func TestNewGetMessagesByDate_StopsWhenPageIsEmpty(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": func(w http.ResponseWriter, r *http.Request) {
			writeRows(w, nil)
		},
	})
	defer ts.Close()

	_, handler := NewGetMessagesByDate(c)
	result, err := handler(context.Background(), makeToolRequest(map[string]any{
		"chat_id": "c1",
		"date":    "2025-01-15",
	}))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	out := decodeByDate(t, resultText(t, result))
	if out.Count != 0 || out.Messages == nil || len(out.Messages) != 0 {
		t.Errorf("expected an empty list, got %+v", out)
	}
}

func TestNewGetMessagesByDate_StopsWhenCursorDoesNotAdvance(t *testing.T) {
	calls := 0
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": func(w http.ResponseWriter, r *http.Request) {
			calls++
			// an API that ignores the cursor keeps returning the same page
			writeRows(w, []byDateRow{{ID: 9, Date: "2025-01-15T12:00:00"}})
		},
	})
	defer ts.Close()

	_, handler := NewGetMessagesByDate(c)
	result, err := handler(context.Background(), makeToolRequest(map[string]any{
		"chat_id": "c1",
		"date":    "2025-01-15",
	}))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	out := decodeByDate(t, resultText(t, result))
	if calls != 2 {
		t.Errorf("expected exactly two calls (first page, then the repeated one), got %d", calls)
	}
	if out.Count != 1 {
		t.Errorf("the repeated page must not be collected twice, count = %d", out.Count)
	}
}

func TestNewGetMessagesByDate_TimezoneShiftsDayBounds(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": func(w http.ResponseWriter, r *http.Request) {
			// 2025-06-10 in Madrid (UTC+2) is 2025-06-09T22:00Z .. 2025-06-10T22:00Z
			if r.URL.Query().Get("before_id") == "" {
				if got := r.URL.Query().Get("before_date"); got != "2025-06-10T22:00:00" {
					t.Errorf("before_date = %q", got)
				}
			}
			writeRows(w, []byDateRow{
				{ID: 3, Date: "2025-06-10T21:59:59"}, // 23:59:59 Madrid: in
				{ID: 2, Date: "2025-06-09T22:30:00"}, // 00:30 Madrid: in
				{ID: 1, Date: "2025-06-09T21:30:00"}, // 23:30 Madrid, previous day: stop
			})
		},
	})
	defer ts.Close()

	_, handler := NewGetMessagesByDate(c)
	result, err := handler(context.Background(), makeToolRequest(map[string]any{
		"chat_id":  "c1",
		"date":     "2025-06-10",
		"timezone": "Europe/Madrid",
	}))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result.IsError {
		t.Fatalf("result is error: %s", resultText(t, result))
	}
	out := decodeByDate(t, resultText(t, result))
	if got := idsOf(t, out.Messages); fmt.Sprint(got) != "[2 3]" {
		t.Errorf("ids = %v, want [2 3]", got)
	}
}

func TestNewGetMessagesByDate_LimitKeepsOldestAndFlagsTruncation(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": func(w http.ResponseWriter, r *http.Request) {
			writeRows(w, []byDateRow{
				{ID: 3, Date: "2025-01-15T15:00:00"},
				{ID: 2, Date: "2025-01-15T12:00:00"},
				{ID: 1, Date: "2025-01-15T09:00:00"},
			})
		},
	})
	defer ts.Close()

	_, handler := NewGetMessagesByDate(c)
	result, err := handler(context.Background(), makeToolRequest(map[string]any{
		"chat_id": "c1",
		"date":    "2025-01-15",
		"limit":   float64(2),
	}))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	out := decodeByDate(t, resultText(t, result))
	if got := idsOf(t, out.Messages); fmt.Sprint(got) != "[1 2]" {
		t.Errorf("ids = %v, want [1 2]", got)
	}
	if !out.Truncated || out.Count != 2 {
		t.Errorf("unexpected envelope: %+v", out)
	}
}

func TestNewGetMessagesByDate_CapsLimit(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": func(w http.ResponseWriter, r *http.Request) {
			writeRows(w, nil)
		},
	})
	defer ts.Close()

	_, handler := NewGetMessagesByDate(c)
	result, err := handler(context.Background(), makeToolRequest(map[string]any{
		"chat_id": "c1",
		"date":    "2025-01-15",
		"limit":   float64(999999),
	}))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result.IsError {
		t.Fatalf("result is error: %s", resultText(t, result))
	}
}

func TestNewGetMessagesByDate_RejectsInvalidTimezone(t *testing.T) {
	ts, c := newTestClient(t, nil)
	defer ts.Close()

	_, handler := NewGetMessagesByDate(c)
	result, err := handler(context.Background(), makeToolRequest(map[string]any{
		"chat_id":  "c1",
		"date":     "2025-01-15",
		"timezone": "Mars/Olympus",
	}))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !result.IsError || !strings.Contains(resultText(t, result), "invalid timezone") {
		t.Errorf("expected an invalid timezone error, got %s", resultText(t, result))
	}
}

func TestNewGetMessagesByDate_ReturnsToolErrorOnAPIFailure(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
	})
	defer ts.Close()

	_, handler := NewGetMessagesByDate(c)
	result, err := handler(context.Background(), makeToolRequest(map[string]any{
		"chat_id": "c1",
		"date":    "2025-01-15",
	}))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !result.IsError {
		t.Error("expected a tool error")
	}
}

func TestNewGetMessagesByDate_ReturnsToolErrorOnBadJSON(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"not":"a list"}`))
		},
	})
	defer ts.Close()

	_, handler := NewGetMessagesByDate(c)
	result, err := handler(context.Background(), makeToolRequest(map[string]any{
		"chat_id": "c1",
		"date":    "2025-01-15",
	}))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !result.IsError || !strings.Contains(resultText(t, result), "unexpected response") {
		t.Errorf("expected an unexpected-response error, got %s", resultText(t, result))
	}
}

func TestNewGetMessagesByDate_ReturnsToolErrorOnBadMessageDate(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": func(w http.ResponseWriter, r *http.Request) {
			writeRows(w, []byDateRow{{ID: 1, Date: "yesterday"}})
		},
	})
	defer ts.Close()

	_, handler := NewGetMessagesByDate(c)
	result, err := handler(context.Background(), makeToolRequest(map[string]any{
		"chat_id": "c1",
		"date":    "2025-01-15",
	}))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !result.IsError || !strings.Contains(resultText(t, result), "unrecognised date") {
		t.Errorf("expected a date error, got %s", resultText(t, result))
	}
}

func TestNewGetMessagesByDate_ReturnsToolErrorOnBadMessageShape(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`[{"id":"not-a-number","date":"2025-01-15T10:00:00"}]`))
		},
	})
	defer ts.Close()

	_, handler := NewGetMessagesByDate(c)
	result, err := handler(context.Background(), makeToolRequest(map[string]any{
		"chat_id": "c1",
		"date":    "2025-01-15",
	}))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !result.IsError || !strings.Contains(resultText(t, result), "unexpected message shape") {
		t.Errorf("expected a shape error, got %s", resultText(t, result))
	}
}

func TestNewGetMessagesByDate_ToolIsReadOnly(t *testing.T) {
	ts, c := newTestClient(t, nil)
	defer ts.Close()

	tool, _ := NewGetMessagesByDate(c)
	if tool.Name != "get_messages_by_date" {
		t.Errorf("tool name = %q", tool.Name)
	}
	if tool.Annotations.ReadOnlyHint == nil || !*tool.Annotations.ReadOnlyHint {
		t.Error("tool should be read-only")
	}
}

func TestParseArchiveTime_AcceptsViewerShapes(t *testing.T) {
	cases := map[string]string{
		"2026-06-10T18:04:17":        "2026-06-10T18:04:17Z",
		"2026-06-10T18:04:17.123456": "2026-06-10T18:04:17.123456Z",
		"2026-06-10 18:04:17":        "2026-06-10T18:04:17Z",
		"2026-06-10T20:04:17+02:00":  "2026-06-10T18:04:17Z",
		"2026-06-10T18:04:17Z":       "2026-06-10T18:04:17Z",
	}
	for in, want := range cases {
		got, err := parseArchiveTime(in)
		if err != nil {
			t.Errorf("%q: %v", in, err)
			continue
		}
		if got.Format("2006-01-02T15:04:05.999999Z07:00") != want {
			t.Errorf("%q -> %s, want %s", in, got.Format("2006-01-02T15:04:05.999999Z07:00"), want)
		}
	}
	if _, err := parseArchiveTime(""); err == nil {
		t.Error("empty date should fail")
	}
	if _, err := parseArchiveTime("10/06/2026"); err == nil {
		t.Error("non-ISO date should fail")
	}
}

func TestNewGetMessagesByDate_RejectsFractionalLimit(t *testing.T) {
	ts, c := newTestClient(t, nil)
	defer ts.Close()

	_, handler := NewGetMessagesByDate(c)
	result, err := handler(context.Background(), makeToolRequest(map[string]any{
		"chat_id": "c1",
		"date":    "2025-01-15",
		"limit":   float64(0.5),
	}))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !result.IsError || !strings.Contains(resultText(t, result), "whole number") {
		t.Errorf("expected a whole-number error, got %s", resultText(t, result))
	}
}

func TestNewGetMessagesByDate_WalksPastTheOldPageCapWithBoundedMemory(t *testing.T) {
	// 120 pages of 3 in-day messages (360 messages), then the previous day.
	const pages = 120
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": func(w http.ResponseWriter, r *http.Request) {
			beforeID := int64(pages*3 + 1)
			if s := r.URL.Query().Get("before_id"); s != "" {
				fmt.Sscanf(s, "%d", &beforeID)
			}
			var rows []byDateRow
			for id := beforeID - 1; id > beforeID-4 && id > 0; id-- {
				rows = append(rows, byDateRow{ID: id, Date: fmt.Sprintf("2025-01-15T%02d:%02d:00", (int(id)/60)%24, int(id)%60)})
			}
			if beforeID-1 <= 3 {
				rows = append(rows, byDateRow{ID: 0, Date: "2025-01-14T23:00:00"})
			}
			writeRows(w, rows)
		},
	})
	defer ts.Close()

	_, handler := NewGetMessagesByDate(c)
	result, err := handler(context.Background(), makeToolRequest(map[string]any{
		"chat_id": "c1",
		"date":    "2025-01-15",
		"limit":   float64(5),
	}))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result.IsError {
		t.Fatalf("result is error: %s", resultText(t, result))
	}
	out := decodeByDate(t, resultText(t, result))
	if got := idsOf(t, out.Messages); fmt.Sprint(got) != "[1 2 3 4 5]" {
		t.Errorf("ids = %v, want the five oldest of the day", got)
	}
	if !out.Truncated || out.Count != 5 {
		t.Errorf("unexpected envelope: %+v", out)
	}
}
