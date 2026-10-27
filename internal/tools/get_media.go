package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"net/http"
	"path"
	"strings"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	// downloadMaxBytes is the largest original get_media downloads. Bigger
	// images are scaled down, so this is far above what it returns.
	downloadMaxBytes = 20 << 20
	// maxPixels refuses to decode an image whose header claims more pixels
	// than this, so a crafted file cannot exhaust memory. Decoding and scaling
	// a 49 MP image allocated about 500 MB; at this cap it stays near 250 MB.
	// A 12 MP phone photo is well within it.
	maxPixels = 25_000_000
	// imageMaxEdge is the longest side of a returned image. The Claude API
	// scales anything longer down to it anyway, and rejects some larger ones.
	imageMaxEdge = 1568
	// imageMaxBytes is the largest returned image. An original within
	// imageMaxEdge but heavier than this is re-encoded as JPEG.
	imageMaxBytes = 2_000_000
)

// scaleSlot lets one image be decoded and scaled at a time, so concurrent calls
// do not multiply the memory a large image takes.
var scaleSlot = make(chan struct{}, 1)

// jpegQualities are tried in order until the encoded image fits imageMaxBytes.
var jpegQualities = []int{85, 70, 50}

// imageMIMETypes are the image formats returned as an image block. The type
// is sniffed from the bytes, never taken from the archive's metadata, because
// a client rejects an image whose declared type does not match its data.
var imageMIMETypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
	"image/gif":  true,
}

// metadataOnlyKinds are media kinds that are a Telegram payload, not a file
// (the viewer's METADATA_ONLY_MEDIA_TYPES). A location or venue can still carry
// a map picture, which then has a url.
var metadataOnlyKinds = map[string]bool{
	"contact": true, "geo": true, "poll": true, "venue": true, "dice": true,
	"invoice": true, "story": true, "giveaway": true, "giveaway_results": true,
	"geo_live": true, "game": true, "unsupported": true,
}

// archivedMedia is the nested media of a message, as the viewer's messages
// route returns it.
type archivedMedia struct {
	Key        *string `json:"id"`
	Kind       *string `json:"type"`
	FileName   *string `json:"file_name"`
	FileSize   *int64  `json:"file_size"`
	MIMEType   *string `json:"mime_type"`
	Downloaded bool    `json:"downloaded"`
	URL        *string `json:"url"`
	NoDownload bool    `json:"no_download"`
	SkipReason *string `json:"skip_reason"`
}

type archivedMessage struct {
	ID           int64          `json:"id"`
	Date         *string        `json:"date"`
	Text         *string        `json:"text"`
	SenderName   *string        `json:"sender_name"`
	FirstName    *string        `json:"first_name"`
	LastName     *string        `json:"last_name"`
	Username     *string        `json:"username"`
	ReplyToMsgID *int64         `json:"reply_to_msg_id"`
	Media        *archivedMedia `json:"media"`
}

// sender names the author the way the viewer does: the stored sender name,
// else the user's first and last name, else the username.
func (m archivedMessage) sender() *string {
	name := strings.TrimSpace(deref(m.SenderName))
	if name == "" {
		name = strings.TrimSpace(deref(m.FirstName) + " " + deref(m.LastName))
	}
	if name == "" {
		name = deref(m.Username)
	}
	if name == "" {
		return nil
	}
	return &name
}

// mediaInfo is the text block of every get_media answer that found a file.
type mediaInfo struct {
	MessageID     int64   `json:"message_id"`
	Date          *string `json:"date"`
	Sender        *string `json:"sender"`
	Text          *string `json:"text"`
	ReplyToMsgID  *int64  `json:"reply_to_msg_id"`
	MediaKey      string  `json:"media_key"`
	Kind          string  `json:"kind"`
	FileName      *string `json:"file_name"`
	MIMEType      *string `json:"mime_type"`
	SizeBytes     *int64  `json:"size_bytes"`
	ImageReturned bool    `json:"image_returned"`
	Width         int     `json:"width,omitempty"`
	Height        int     `json:"height,omitempty"`
	Scaled        bool    `json:"scaled,omitempty"`
	Note          string  `json:"note,omitempty"`
}

const mediaOutputSchema = `{"type":"object","properties":{` +
	`"message_id":{"type":"integer"},"date":{"type":["string","null"],"description":"Naive UTC."},` +
	`"sender":{"type":["string","null"]},"text":{"type":["string","null"],"description":"The caption."},` +
	`"reply_to_msg_id":{"type":["integer","null"]},` +
	`"media_key":{"type":"string"},"kind":{"type":"string","description":"photo, document, video, geo..."},` +
	`"file_name":{"type":["string","null"]},"mime_type":{"type":["string","null"]},"size_bytes":{"type":["integer","null"]},` +
	`"image_returned":{"type":"boolean","description":"True when an image content block follows."},` +
	`"width":{"type":"integer"},"height":{"type":"integer"},"scaled":{"type":"boolean"},` +
	`"note":{"type":"string","description":"Why the file is not included, when it is not."}},` +
	`"required":["message_id","media_key","kind","image_returned"]}`

func NewGetMedia(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {

	tool := mcp.NewTool("get_media",
		mcp.WithDescription("Get the photo or file attached to one message. A JPEG, PNG, WebP or GIF image comes back as an image you can look at, scaled to at most 1568 pixels on its longest side, after a JSON text block with the message's message_id, date, sender, text (the caption) and reply_to_msg_id, and the file's media_key, kind, file_name, mime_type, size_bytes, image_returned, width, height and scaled. Any other file returns only that JSON text block, with a note saying why the file is not included."),
		describe("Look at a message's photo", true),
		chatIDParam(),
		mcp.WithInteger("message_id",
			mcp.Required(), mcp.Min(1),
			mcp.Description("Id of the message that carries the media; for a reply such as \"see my picture\", its reply_to_msg_id."),
		),
		mcp.WithRawOutputSchema([]byte(mediaOutputSchema)),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		chatID, err := req.RequireString("chat_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		// The lookup asks for before_id = message_id + 1, which must stay below 2^53.
		messageID, present, err := wholeNumberArg(req.GetArguments(), "message_id", 1<<53-2)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if !present || messageID < 1 {
			return mcp.NewToolResultError("message_id is required and must be at least 1"), nil
		}

		// The viewer has no single-message route. A lone before_id is ordered by
		// id, newest first, so before_id = N+1 with limit 1 returns message N
		// when it exists.
		body, err := c.GetMessagesCursor(ctx, chatID, 1, client.MessagesCursor{BeforeID: messageID + 1})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		var messages []archivedMessage
		if err := json.Unmarshal(body, &messages); err != nil {
			return mcp.NewToolResultError("unexpected messages response: " + err.Error()), nil
		}
		if len(messages) == 0 || messages[0].ID != messageID {
			return mcp.NewToolResultError(fmt.Sprintf("message %d not found in this chat", messageID)), nil
		}
		media := messages[0].Media
		if media == nil {
			return mcp.NewToolResultError(fmt.Sprintf("message %d has no media", messageID)), nil
		}
		kind := deref(media.Kind)
		if media.URL == nil && metadataOnlyKinds[kind] {
			return mcp.NewToolResultError(fmt.Sprintf("message %d has a %s, which has no file; get_messages shows its details", messageID, kind)), nil
		}
		if media.NoDownload {
			return mcp.NewToolResultError(fmt.Sprintf("message %d has a %s, but downloads are disabled for this archive login", messageID, kind)), nil
		}
		if media.URL == nil || media.Key == nil || *media.Key == "" {
			if !media.Downloaded {
				msg := fmt.Sprintf("message %d has a %s that the archive has not downloaded", messageID, kind)
				if r := deref(media.SkipReason); r != "" {
					msg += " (skip reason: " + r + ")"
				}
				return mcp.NewToolResultError(msg), nil
			}
			return mcp.NewToolResultError(fmt.Sprintf("message %d has a %s whose file is missing from the archive", messageID, kind)), nil
		}

		msg := messages[0]
		info := mediaInfo{
			MessageID:    messageID,
			Date:         msg.Date,
			Sender:       msg.sender(),
			Text:         msg.Text,
			ReplyToMsgID: msg.ReplyToMsgID,
			MediaKey:     *media.Key,
			Kind:         kind,
			FileName:     media.FileName,
			MIMEType:     media.MIMEType,
			SizeBytes:    media.FileSize,
		}
		if !looksLikeImage(media) {
			info.Note = "not an image; the file itself is not returned"
			return infoResult(info), nil
		}
		tooBig := fmt.Sprintf("image file larger than %d bytes; not returned", downloadMaxBytes)
		if media.FileSize != nil && *media.FileSize > downloadMaxBytes {
			info.Note = tooBig
			return infoResult(info), nil
		}

		data, err := c.GetMedia(ctx, chatID, *media.Key, downloadMaxBytes)
		var tooLarge *client.TooLargeError
		if errors.As(err, &tooLarge) {
			info.Note = tooBig
			return infoResult(info), nil
		}
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		size := int64(len(data))
		info.SizeBytes = &size
		sniffed := http.DetectContentType(data)
		if !imageMIMETypes[sniffed] {
			info.Note = "the file is not a JPEG, PNG, WebP or GIF image; not returned"
			return infoResult(info), nil
		}
		info.MIMEType = &sniffed

		out, outType, w, h, scaled, note := fitImage(data, sniffed)
		if note != "" {
			info.Note = note
			return infoResult(info), nil
		}
		info.ImageReturned = true
		info.Width, info.Height, info.Scaled = w, h, scaled
		res := mcp.NewToolResultImage(infoText(info), base64.StdEncoding.EncodeToString(out), outType)
		res.StructuredContent = info
		return res, nil
	}

	return tool, handler
}

// fitImage returns the image to send: the original when it is within
// imageMaxEdge and imageMaxBytes, otherwise a JPEG scaled to fit. A non-empty
// note means no image can be sent and says why.
func fitImage(data []byte, mimeType string) (out []byte, outType string, width, height int, scaled bool, note string) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, "", 0, 0, false, "the image could not be read; not returned"
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > maxPixels {
		return nil, "", 0, 0, false, fmt.Sprintf("image of %dx%d pixels is too large to scale; not returned", cfg.Width, cfg.Height)
	}
	// A JPEG straight from a camera can be stored sideways with an EXIF tag
	// saying how to turn it. Decoding drops that tag, and not every client
	// reads it, so such an image is always re-encoded upright.
	orientation := 1
	if mimeType == "image/jpeg" {
		orientation = jpegOrientation(data)
	}
	if orientation == 1 && max(cfg.Width, cfg.Height) <= imageMaxEdge && len(data) <= imageMaxBytes {
		return data, mimeType, cfg.Width, cfg.Height, false, ""
	}

	scaleSlot <- struct{}{}
	defer func() { <-scaleSlot }()
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", 0, 0, false, "the image could not be read; not returned"
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if long := max(w, h); long > imageMaxEdge {
		w = max(1, w*imageMaxEdge/long)
		h = max(1, h*imageMaxEdge/long)
	}
	// JPEG has no transparency: paint on white so a transparent area does not
	// turn black.
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	dst = orient(dst, orientation)
	w, h = dst.Bounds().Dx(), dst.Bounds().Dy()

	for _, q := range jpegQualities {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: q}); err != nil {
			return nil, "", 0, 0, false, "the image could not be re-encoded; not returned"
		}
		if buf.Len() <= imageMaxBytes {
			return buf.Bytes(), "image/jpeg", w, h, true, ""
		}
	}
	return nil, "", 0, 0, false, fmt.Sprintf("image still larger than %d bytes after scaling; not returned", imageMaxBytes)
}

// jpegOrientation returns the EXIF orientation (1 to 8) of a JPEG, or 1 when
// it has none or the tag cannot be read.
func jpegOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 1
	}
	for i := 2; i+4 <= len(data); {
		if data[i] != 0xFF {
			return 1
		}
		marker := data[i+1]
		if marker == 0xDA || marker == 0xD9 { // image data or end: no EXIF before it
			return 1
		}
		size := int(binary.BigEndian.Uint16(data[i+2 : i+4]))
		end := i + 2 + size
		if size < 2 || end > len(data) {
			return 1
		}
		if marker == 0xE1 {
			if o := exifOrientation(data[i+4 : end]); o != 0 {
				return o
			}
		}
		i = end
	}
	return 1
}

// exifOrientation reads tag 0x0112 from IFD0 of an APP1 Exif segment, or 0.
func exifOrientation(seg []byte) int {
	if len(seg) < 14 || string(seg[:6]) != "Exif\x00\x00" {
		return 0
	}
	tiff := seg[6:]
	var bo binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 0
	}
	ifd := int(bo.Uint32(tiff[4:8]))
	if ifd < 8 || ifd+2 > len(tiff) {
		return 0
	}
	n := int(bo.Uint16(tiff[ifd : ifd+2]))
	for e := 0; e < n; e++ {
		at := ifd + 2 + 12*e
		if at+12 > len(tiff) {
			return 0
		}
		if bo.Uint16(tiff[at:at+2]) == 0x0112 {
			if o := int(bo.Uint16(tiff[at+8 : at+10])); o >= 1 && o <= 8 {
				return o
			}
			return 0
		}
	}
	return 0
}

// orient turns an image stored with EXIF orientation o upright.
func orient(src *image.RGBA, o int) *image.RGBA {
	if o < 2 || o > 8 {
		return src
	}
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	ow, oh := w, h
	if o >= 5 {
		ow, oh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, ow, oh))
	for y := 0; y < oh; y++ {
		for x := 0; x < ow; x++ {
			var sx, sy int
			switch o {
			case 2:
				sx, sy = w-1-x, y
			case 3:
				sx, sy = w-1-x, h-1-y
			case 4:
				sx, sy = x, h-1-y
			case 5:
				sx, sy = y, x
			case 6:
				sx, sy = y, h-1-x
			case 7:
				sx, sy = w-1-y, h-1-x
			case 8:
				sx, sy = w-1-y, x
			}
			dst.SetRGBA(x, y, src.RGBAAt(sx, sy))
		}
	}
	return dst
}

// looksLikeImage decides from the metadata whether the file is worth
// downloading as an image. The bytes have the final say.
func looksLikeImage(m *archivedMedia) bool {
	if deref(m.Kind) == "photo" || strings.HasPrefix(deref(m.MIMEType), "image/") {
		return true
	}
	switch strings.ToLower(path.Ext(deref(m.FileName))) {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
		return true
	}
	return false
}

func infoText(info mediaInfo) string {
	b, _ := json.Marshal(info)
	return string(b)
}

func infoResult(info mediaInfo) *mcp.CallToolResult {
	return mcp.NewToolResultStructured(info, infoText(info))
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
