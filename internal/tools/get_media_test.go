package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"strings"
	"testing"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// tinyPNG is a real 1x1 PNG, so content sniffing sees an image.
func tinyPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// messageRoute answers the one-message lookup get_media makes with the given
// JSON array, and checks the query that selects message 42.
func messageRoute(t *testing.T, payload string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("before_id") != "43" || q.Get("limit") != "1" {
			t.Errorf("lookup query = %q, want before_id=43&limit=1", r.URL.RawQuery)
		}
		w.Write([]byte(payload))
	}
}

func mediaInfoOf(t *testing.T, result *mcp.CallToolResult) mediaInfo {
	t.Helper()
	var info mediaInfo
	if err := json.Unmarshal([]byte(resultText(t, result)), &info); err != nil {
		t.Fatalf("text block is not the media JSON: %v: %s", err, resultText(t, result))
	}
	return info
}

func callGetMedia(t *testing.T, c *client.Client, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	_, handler := NewGetMedia(c)
	result, err := handler(context.Background(), makeToolRequest(args))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	return result
}

func TestNewGetMedia_ReturnsImageBlock(t *testing.T) {
	img := tinyPNG(t)
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": messageRoute(t, `[{"id":42,"text":"look","media":{"id":"42_photo","type":"photo","file_name":"42_photo.png","file_size":`+
			itoa(len(img))+`,"mime_type":null,"downloaded":true,"url":"/media/c1/42_photo"}}]`),
		"/media/c1/42_photo": func(w http.ResponseWriter, r *http.Request) {
			w.Write(img)
		},
	})
	defer ts.Close()

	result := callGetMedia(t, c, map[string]any{"chat_id": "c1", "message_id": float64(42)})
	if result.IsError {
		t.Fatalf("result is error: %s", resultText(t, result))
	}
	if len(result.Content) != 2 {
		t.Fatalf("content blocks = %d, want text + image", len(result.Content))
	}
	info := mediaInfoOf(t, result)
	if !info.ImageReturned || info.MediaKey != "42_photo" || info.Kind != "photo" || info.MessageID != 42 {
		t.Errorf("unexpected info: %+v", info)
	}
	if info.MIMEType == nil || *info.MIMEType != "image/png" {
		t.Errorf("mime_type = %v, want image/png", info.MIMEType)
	}
	if info.SizeBytes == nil || *info.SizeBytes != int64(len(img)) {
		t.Errorf("size_bytes = %v, want %d", info.SizeBytes, len(img))
	}
	ic, ok := mcp.AsImageContent(result.Content[1])
	if !ok {
		t.Fatalf("second block is not an image: %#v", result.Content[1])
	}
	if ic.MIMEType != "image/png" {
		t.Errorf("image mimeType = %q, want image/png", ic.MIMEType)
	}
	got, err := base64.StdEncoding.DecodeString(ic.Data)
	if err != nil || !bytes.Equal(got, img) {
		t.Errorf("image data does not round-trip (err %v)", err)
	}
}

// The archive's metadata can name the wrong type; a client rejects an image
// whose declared type does not match its bytes, so the bytes decide.
func TestNewGetMedia_TypeComesFromTheBytes(t *testing.T) {
	img := tinyPNG(t)
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": messageRoute(t, `[{"id":42,"media":{"id":"42_photo","type":"photo","file_name":"a.jpg","mime_type":"image/jpeg","downloaded":true,"url":"/media/c1/42_photo"}}]`),
		"/media/c1/42_photo": func(w http.ResponseWriter, r *http.Request) {
			w.Write(img)
		},
	})
	defer ts.Close()

	result := callGetMedia(t, c, map[string]any{"chat_id": "c1", "message_id": float64(42)})
	if result.IsError || len(result.Content) != 2 {
		t.Fatalf("expected text + image, got %d blocks (error %v)", len(result.Content), result.IsError)
	}
	ic, _ := mcp.AsImageContent(result.Content[1])
	if ic == nil || ic.MIMEType != "image/png" {
		t.Errorf("image mimeType = %v, want image/png from the bytes", ic)
	}
}

func TestNewGetMedia_NonImageFileReturnsTextOnly(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": messageRoute(t, `[{"id":42,"media":{"id":"42_document","type":"document","file_name":"report.pdf","file_size":2048,"mime_type":"application/pdf","downloaded":true,"url":"/media/c1/42_document"}}]`),
		"/media/c1/42_document": func(w http.ResponseWriter, r *http.Request) {
			t.Error("a non-image file must not be downloaded")
		},
	})
	defer ts.Close()

	result := callGetMedia(t, c, map[string]any{"chat_id": "c1", "message_id": float64(42)})
	if result.IsError {
		t.Fatalf("a non-image file is an answer, not an error: %s", resultText(t, result))
	}
	if len(result.Content) != 1 {
		t.Fatalf("content blocks = %d, want text only", len(result.Content))
	}
	info := mediaInfoOf(t, result)
	if info.ImageReturned || info.Kind != "document" || info.Note == "" {
		t.Errorf("unexpected info: %+v", info)
	}
	if info.FileName == nil || *info.FileName != "report.pdf" || info.MIMEType == nil || *info.MIMEType != "application/pdf" || info.SizeBytes == nil || *info.SizeBytes != 2048 {
		t.Errorf("metadata not passed on: %+v", info)
	}
}

func TestNewGetMedia_ImageOverCapByMetadataIsNotDownloaded(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": messageRoute(t, `[{"id":42,"media":{"id":"42_document","type":"document","file_name":"big.png","file_size":`+
			itoa(imageMaxBytes+1)+`,"mime_type":"image/png","downloaded":true,"url":"/media/c1/42_document"}}]`),
		"/media/c1/42_document": func(w http.ResponseWriter, r *http.Request) {
			t.Error("an image over the cap must not be downloaded")
		},
	})
	defer ts.Close()

	result := callGetMedia(t, c, map[string]any{"chat_id": "c1", "message_id": float64(42)})
	if result.IsError || len(result.Content) != 1 {
		t.Fatalf("expected one text block, got %d (error %v)", len(result.Content), result.IsError)
	}
	if info := mediaInfoOf(t, result); info.ImageReturned || !strings.Contains(info.Note, "larger than") {
		t.Errorf("unexpected info: %+v", info)
	}
}

// file_size can be missing or wrong. The download itself stops at the cap, and
// the answer is the note, never a truncated image.
func TestNewGetMedia_ImageOverCapByBytesIsNotReturned(t *testing.T) {
	big := append(tinyPNG(t), make([]byte, imageMaxBytes)...)
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": messageRoute(t, `[{"id":42,"media":{"id":"42_photo","type":"photo","file_size":10,"downloaded":true,"url":"/media/c1/42_photo"}}]`),
		"/media/c1/42_photo": func(w http.ResponseWriter, r *http.Request) {
			w.Write(big)
		},
	})
	defer ts.Close()

	result := callGetMedia(t, c, map[string]any{"chat_id": "c1", "message_id": float64(42)})
	if result.IsError || len(result.Content) != 1 {
		t.Fatalf("expected one text block, got %d (error %v)", len(result.Content), result.IsError)
	}
	if info := mediaInfoOf(t, result); info.ImageReturned || !strings.Contains(info.Note, "larger than") {
		t.Errorf("unexpected info: %+v", info)
	}
}

func TestNewGetMedia_ImageAtCapIsReturned(t *testing.T) {
	img := append(tinyPNG(t), make([]byte, imageMaxBytes)...)[:imageMaxBytes]
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": messageRoute(t, `[{"id":42,"media":{"id":"42_photo","type":"photo","downloaded":true,"url":"/media/c1/42_photo"}}]`),
		"/media/c1/42_photo": func(w http.ResponseWriter, r *http.Request) {
			w.Write(img)
		},
	})
	defer ts.Close()

	result := callGetMedia(t, c, map[string]any{"chat_id": "c1", "message_id": float64(42)})
	if result.IsError || len(result.Content) != 2 {
		t.Fatalf("expected text + image, got %d blocks (error %v)", len(result.Content), result.IsError)
	}
	ic, _ := mcp.AsImageContent(result.Content[1])
	if ic == nil || len(ic.Data) > 5*1024*1024 {
		t.Errorf("base64 must stay within 5 MiB, got %d bytes", len(ic.Data))
	}
}

func TestNewGetMedia_BytesThatAreNotAnImage(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": messageRoute(t, `[{"id":42,"media":{"id":"42_photo","type":"photo","downloaded":true,"url":"/media/c1/42_photo"}}]`),
		"/media/c1/42_photo": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("<html>not a picture</html>"))
		},
	})
	defer ts.Close()

	result := callGetMedia(t, c, map[string]any{"chat_id": "c1", "message_id": float64(42)})
	if result.IsError || len(result.Content) != 1 {
		t.Fatalf("expected one text block, got %d (error %v)", len(result.Content), result.IsError)
	}
	if info := mediaInfoOf(t, result); info.ImageReturned || info.Note == "" {
		t.Errorf("unexpected info: %+v", info)
	}
}

func TestNewGetMedia_ErrorAnswers(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		want    string
	}{
		{"message not found", `[{"id":41,"media":null}]`, "message 42 not found"},
		{"empty chat", `[]`, "message 42 not found"},
		{"no media", `[{"id":42,"text":"hi","media":null}]`, "has no media"},
		{"downloads off", `[{"id":42,"media":{"id":"42_photo","type":"photo","file_path":null,"url":null,"downloaded":false,"no_download":true}}]`, "downloads are disabled"},
		{"not downloaded", `[{"id":42,"media":{"id":"42_video","type":"video","url":null,"downloaded":false,"skip_reason":"oversize"}}]`, "not downloaded (skip reason: oversize)"},
		{"kind without a file", `[{"id":42,"media":{"id":"42_geo","type":"geo","url":null,"downloaded":true}}]`, "geo, which has no file"},
		{"contact is never downloaded", `[{"id":42,"media":{"id":"42_contact","type":"contact","url":null,"downloaded":false}}]`, "contact, which has no file"},
		{"file missing on disk", `[{"id":42,"media":{"id":"42_photo","type":"photo","url":null,"downloaded":true}}]`, "file is missing"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts, c := newTestClient(t, map[string]http.HandlerFunc{
				"/api/chats/c1/messages": messageRoute(t, tc.payload),
				"/media/": func(w http.ResponseWriter, r *http.Request) {
					t.Error("no file must be requested")
				},
			})
			defer ts.Close()

			result := callGetMedia(t, c, map[string]any{"chat_id": "c1", "message_id": float64(42)})
			if !result.IsError {
				t.Fatalf("expected IsError, got %s", resultText(t, result))
			}
			if text := resultText(t, result); !strings.Contains(text, tc.want) {
				t.Errorf("text = %q, want it to contain %q", text, tc.want)
			}
		})
	}
}

func TestNewGetMedia_ReturnsToolErrorWhenFileRefused(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": messageRoute(t, `[{"id":42,"media":{"id":"42_photo","type":"photo","downloaded":true,"url":"/media/c1/42_photo"}}]`),
		"/media/c1/42_photo": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"detail":"Downloads disabled for this account"}`))
		},
	})
	defer ts.Close()

	result := callGetMedia(t, c, map[string]any{"chat_id": "c1", "message_id": float64(42)})
	if !result.IsError || !strings.Contains(resultText(t, result), "403") {
		t.Errorf("expected a 403 tool error, got %s", resultText(t, result))
	}
}

func TestNewGetMedia_ReturnsToolErrorOnAPIFailure(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"detail":"Chat not found"}`))
		},
	})
	defer ts.Close()

	result := callGetMedia(t, c, map[string]any{"chat_id": "c1", "message_id": float64(42)})
	if !result.IsError || !strings.Contains(resultText(t, result), "Chat not found") {
		t.Errorf("expected the viewer's 404, got %s", resultText(t, result))
	}
}

func TestNewGetMedia_RejectsBadArguments(t *testing.T) {
	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{"missing chat_id", map[string]any{"message_id": float64(42)}, "chat_id"},
		{"missing message_id", map[string]any{"chat_id": "c1"}, "message_id is required"},
		{"zero message_id", map[string]any{"chat_id": "c1", "message_id": float64(0)}, "at least 1"},
		{"fractional message_id", map[string]any{"chat_id": "c1", "message_id": float64(4.5)}, "whole number"},
		{"string message_id", map[string]any{"chat_id": "c1", "message_id": "42"}, "must be a number"},
		{"huge message_id", map[string]any{"chat_id": "c1", "message_id": float64(1 << 53)}, "between 0 and"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts, c := newTestClient(t, map[string]http.HandlerFunc{
				"/api/chats/": func(w http.ResponseWriter, r *http.Request) {
					t.Error("the archive must not be called with invalid arguments")
				},
			})
			defer ts.Close()

			result := callGetMedia(t, c, tc.args)
			if !result.IsError {
				t.Fatalf("expected IsError, got %s", resultText(t, result))
			}
			if text := resultText(t, result); !strings.Contains(text, tc.want) {
				t.Errorf("text = %q, want it to contain %q", text, tc.want)
			}
		})
	}
}

func TestNewGetMedia_ToolDefinition(t *testing.T) {
	c := client.New("http://fake", "", "")
	tool, _ := NewGetMedia(c)
	if tool.Name != "get_media" {
		t.Errorf("tool name = %q, want get_media", tool.Name)
	}
	if tool.Annotations.ReadOnlyHint == nil || !*tool.Annotations.ReadOnlyHint {
		t.Error("get_media should be read-only")
	}
	required := strings.Join(tool.InputSchema.Required, ",")
	if required != "chat_id,message_id" {
		t.Errorf("required = %q, want chat_id,message_id", required)
	}
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}
