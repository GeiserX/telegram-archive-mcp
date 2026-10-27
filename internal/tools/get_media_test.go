package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math/rand/v2"
	"net/http"
	"os"
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
		t.Errorf("a small image must be sent as is (err %v)", err)
	}
	if info.Scaled || info.Width != 1 || info.Height != 1 {
		t.Errorf("unexpected size info: %+v", info)
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
			itoa(downloadMaxBytes+1)+`,"mime_type":"image/png","downloaded":true,"url":"/media/c1/42_document"}}]`),
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
	big := append(tinyPNG(t), make([]byte, downloadMaxBytes)...)
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

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// serveImage runs get_media for message 42 whose photo is data.
func serveImage(t *testing.T, data []byte) *mcp.CallToolResult {
	t.Helper()
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": messageRoute(t, `[{"id":42,"media":{"id":"42_photo","type":"photo","downloaded":true,"url":"/media/c1/42_photo"}}]`),
		"/media/c1/42_photo": func(w http.ResponseWriter, r *http.Request) {
			w.Write(data)
		},
	})
	t.Cleanup(ts.Close)
	return callGetMedia(t, c, map[string]any{"chat_id": "c1", "message_id": float64(42)})
}

// returnedImage decodes the image block of a two-block answer.
func returnedImage(t *testing.T, result *mcp.CallToolResult) (*mcp.ImageContent, image.Image, []byte) {
	t.Helper()
	if result.IsError || len(result.Content) != 2 {
		t.Fatalf("expected text + image, got %d blocks: %s", len(result.Content), resultText(t, result))
	}
	ic, ok := mcp.AsImageContent(result.Content[1])
	if !ok {
		t.Fatalf("second block is not an image")
	}
	raw, err := base64.StdEncoding.DecodeString(ic.Data)
	if err != nil {
		t.Fatalf("base64: %v", err)
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("returned image does not decode: %v", err)
	}
	return ic, img, raw
}

// A phone photo is commonly 2560 pixels wide. The API scales anything past
// 1568 down anyway, and a gateway may drop an image past 2000, so it is sent
// scaled to 1568 on its longest side.
func TestNewGetMedia_ScalesALargeImage(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 3000, 1000))
	draw.Draw(src, src.Bounds(), image.NewUniform(color.RGBA{200, 30, 30, 255}), image.Point{}, draw.Src)
	result := serveImage(t, encodePNG(t, src))

	ic, img, raw := returnedImage(t, result)
	if ic.MIMEType != "image/jpeg" || http.DetectContentType(raw) != "image/jpeg" {
		t.Errorf("mimeType = %q, bytes %q, want image/jpeg", ic.MIMEType, http.DetectContentType(raw))
	}
	if b := img.Bounds(); b.Dx() != 1568 || b.Dy() != 522 {
		t.Errorf("returned %dx%d, want 1568x522", b.Dx(), b.Dy())
	}
	info := mediaInfoOf(t, result)
	if !info.Scaled || info.Width != 1568 || info.Height != 522 {
		t.Errorf("unexpected info: %+v", info)
	}
	if info.MIMEType == nil || *info.MIMEType != "image/png" {
		t.Errorf("mime_type should name the original file, got %v", info.MIMEType)
	}
}

// An image small in pixels can still be heavy in bytes (an uncompressible PNG);
// it is re-encoded so the returned image stays within imageMaxBytes.
func TestNewGetMedia_ReencodesAHeavyImage(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 1000, 1000))
	rng := rand.New(rand.NewPCG(1, 2))
	for i := range src.Pix {
		src.Pix[i] = byte(rng.Uint32())
		if i%4 == 3 {
			src.Pix[i] = 255
		}
	}
	data := encodePNG(t, src)
	if len(data) <= imageMaxBytes {
		t.Fatalf("fixture is only %d bytes; it must exceed imageMaxBytes", len(data))
	}

	ic, img, raw := returnedImage(t, serveImage(t, data))
	if ic.MIMEType != "image/jpeg" || len(raw) > imageMaxBytes {
		t.Errorf("got %s of %d bytes, want a JPEG within %d", ic.MIMEType, len(raw), imageMaxBytes)
	}
	if b := img.Bounds(); b.Dx() != 1000 || b.Dy() != 1000 {
		t.Errorf("returned %dx%d, want 1000x1000", b.Dx(), b.Dy())
	}
}

func TestNewGetMedia_TransparencyBecomesWhite(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 2000, 20))
	_, img, _ := returnedImage(t, serveImage(t, encodePNG(t, src)))
	r, g, b, _ := img.At(10, 5).RGBA()
	if r>>8 < 240 || g>>8 < 240 || b>>8 < 240 {
		t.Errorf("transparent pixel became (%d,%d,%d), want white", r>>8, g>>8, b>>8)
	}
}

// A header that claims a huge image is refused before decoding, so a crafted
// file cannot make the server allocate gigabytes.
func TestNewGetMedia_RefusesAPixelBomb(t *testing.T) {
	data := tinyPNG(t)
	binary.BigEndian.PutUint32(data[16:20], 20000)
	binary.BigEndian.PutUint32(data[20:24], 20000)
	binary.BigEndian.PutUint32(data[29:33], crc32.ChecksumIEEE(data[12:29]))

	result := serveImage(t, data)
	if result.IsError || len(result.Content) != 1 {
		t.Fatalf("expected one text block, got %d (error %v)", len(result.Content), result.IsError)
	}
	if info := mediaInfoOf(t, result); info.ImageReturned || !strings.Contains(info.Note, "20000x20000") {
		t.Errorf("unexpected info: %+v", info)
	}
}

// The text block carries what an agent needs to talk about the photo: when it
// was sent, by whom, its caption and the message it answered.
func TestNewGetMedia_TextBlockCarriesTheMessage(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats/c1/messages": messageRoute(t, `[{"id":42,"date":"2026-06-10T18:04:17","text":"our new kitchen","sender_name":null,"first_name":"Ada","last_name":"Lovelace","username":"ada","reply_to_msg_id":40,
			"media":{"id":"42_document","type":"document","file_name":"plan.pdf","mime_type":"application/pdf","downloaded":true,"url":"/media/c1/42_document"}}]`),
	})
	defer ts.Close()

	info := mediaInfoOf(t, callGetMedia(t, c, map[string]any{"chat_id": "c1", "message_id": float64(42)}))
	if deref(info.Date) != "2026-06-10T18:04:17" || deref(info.Sender) != "Ada Lovelace" || deref(info.Text) != "our new kitchen" {
		t.Errorf("message fields lost: %+v", info)
	}
	if info.ReplyToMsgID == nil || *info.ReplyToMsgID != 40 {
		t.Errorf("reply_to_msg_id = %v, want 40", info.ReplyToMsgID)
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
	// The viewer's body never reaches the model; the status picks a fixed sentence.
	if !result.IsError || !strings.Contains(resultText(t, result), "404") || strings.Contains(resultText(t, result), "Chat not found") {
		t.Errorf("expected a fixed 404 sentence, got %s", resultText(t, result))
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

// halves is a w x h image, left half a, right half b.
func halves(w, h int, a, b color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, image.Rect(0, 0, w/2, h), image.NewUniform(a), image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(w/2, 0, w, h), image.NewUniform(b), image.Point{}, draw.Src)
	return img
}

var (
	red  = color.RGBA{255, 0, 0, 255}
	blue = color.RGBA{0, 0, 255, 255}
)

// isNear reports whether the pixel at (x, y) is close to want, allowing for
// JPEG loss.
func isNear(img image.Image, x, y int, want color.RGBA) bool {
	r, g, b, _ := img.At(x, y).RGBA()
	d := func(a uint32, b uint8) bool { return int(a>>8)-int(b) < 60 && int(b)-int(a>>8) < 60 }
	return d(r, want.R) && d(g, want.G) && d(b, want.B)
}

// jpegWithOrientation encodes img as a JPEG carrying an EXIF orientation tag.
func jpegWithOrientation(t *testing.T, img image.Image, o uint16) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	tiff := []byte("MM\x00\x2a\x00\x00\x00\x08\x00\x01\x01\x12\x00\x03\x00\x00\x00\x01")
	tiff = append(tiff, byte(o>>8), byte(o), 0, 0, 0, 0, 0, 0)
	seg := append([]byte("Exif\x00\x00"), tiff...)
	app1 := []byte{0xFF, 0xE1, byte((len(seg) + 2) >> 8), byte(len(seg) + 2)}
	app1 = append(app1, seg...)
	data := buf.Bytes()
	return append(append(append([]byte{}, data[:2]...), app1...), data[2:]...)
}

// A camera JPEG stored sideways is turned upright, even when it is small
// enough to send as is: decoding drops the EXIF tag, and not every client
// reads it.
func TestNewGetMedia_TurnsAnExifRotatedJPEGUpright(t *testing.T) {
	cases := []struct {
		o          uint16
		w, h       int
		redAt      image.Point
		blueAt     image.Point
		orientName string
	}{
		{6, 20, 40, image.Pt(10, 5), image.Pt(10, 35), "rotate 90 clockwise"},
		{8, 20, 40, image.Pt(10, 35), image.Pt(10, 5), "rotate 90 counter-clockwise"},
		{3, 40, 20, image.Pt(35, 10), image.Pt(5, 10), "rotate 180"},
	}
	for _, tc := range cases {
		t.Run(tc.orientName, func(t *testing.T) {
			data := jpegWithOrientation(t, halves(40, 20, red, blue), tc.o)
			result := serveImage(t, data)
			_, img, _ := returnedImage(t, result)
			if b := img.Bounds(); b.Dx() != tc.w || b.Dy() != tc.h {
				t.Fatalf("returned %dx%d, want %dx%d", b.Dx(), b.Dy(), tc.w, tc.h)
			}
			if !isNear(img, tc.redAt.X, tc.redAt.Y, red) || !isNear(img, tc.blueAt.X, tc.blueAt.Y, blue) {
				t.Errorf("image not turned upright: %v at %v, %v at %v", img.At(tc.redAt.X, tc.redAt.Y), tc.redAt, img.At(tc.blueAt.X, tc.blueAt.Y), tc.blueAt)
			}
			if info := mediaInfoOf(t, result); !info.Scaled || info.Width != tc.w || info.Height != tc.h {
				t.Errorf("unexpected info: %+v", info)
			}
		})
	}
}

func TestNewGetMedia_SmallUprightJPEGIsSentAsIs(t *testing.T) {
	data := jpegWithOrientation(t, halves(40, 20, red, blue), 1)
	_, _, raw := returnedImage(t, serveImage(t, data))
	if !bytes.Equal(raw, data) {
		t.Error("an upright JPEG within the limits must be sent byte for byte")
	}
}

func TestNewGetMedia_ScalesAGIF(t *testing.T) {
	src := image.NewPaletted(image.Rect(0, 0, 2000, 10), color.Palette{red, blue})
	draw.Draw(src, image.Rect(1000, 0, 2000, 10), image.NewUniform(blue), image.Point{}, draw.Src)
	draw.Draw(src, image.Rect(0, 0, 1000, 10), image.NewUniform(red), image.Point{}, draw.Src)
	var buf bytes.Buffer
	if err := gif.Encode(&buf, src, nil); err != nil {
		t.Fatalf("encode gif: %v", err)
	}
	ic, img, _ := returnedImage(t, serveImage(t, buf.Bytes()))
	if ic.MIMEType != "image/jpeg" || img.Bounds().Dx() != 1568 || img.Bounds().Dy() != 7 {
		t.Errorf("got %s %v, want image/jpeg 1568x7", ic.MIMEType, img.Bounds())
	}
	if !isNear(img, 100, 3, red) || !isNear(img, 1400, 3, blue) {
		t.Error("GIF content lost in scaling")
	}
}

// testdata/wide.webp is a lossless 2000x100 WebP: left half opaque red, right
// half fully transparent.
func TestNewGetMedia_ScalesAWebPWithAlpha(t *testing.T) {
	data, err := os.ReadFile("testdata/wide.webp")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	ic, img, _ := returnedImage(t, serveImage(t, data))
	if ic.MIMEType != "image/jpeg" || img.Bounds().Dx() != 1568 || img.Bounds().Dy() != 78 {
		t.Errorf("got %s %v, want image/jpeg 1568x78", ic.MIMEType, img.Bounds())
	}
	if !isNear(img, 100, 40, red) || !isNear(img, 1400, 40, color.RGBA{255, 255, 255, 255}) {
		t.Errorf("want red left and white right, got %v and %v", img.At(100, 40), img.At(1400, 40))
	}
}
