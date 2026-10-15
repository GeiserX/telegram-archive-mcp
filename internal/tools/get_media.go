package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// imageMaxBytes is the largest image get_media returns. Its base64 is
// 4 * 3_932_160 / 3 = 5_242_880 bytes, the Claude API's 5 MB per-image limit.
const imageMaxBytes = 3_932_160

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
	ID    int64          `json:"id"`
	Media *archivedMedia `json:"media"`
}

// mediaInfo is the text block of every get_media answer that found a file.
type mediaInfo struct {
	MessageID     int64   `json:"message_id"`
	MediaKey      string  `json:"media_key"`
	Kind          string  `json:"kind"`
	FileName      *string `json:"file_name"`
	MIMEType      *string `json:"mime_type"`
	SizeBytes     *int64  `json:"size_bytes"`
	ImageReturned bool    `json:"image_returned"`
	Note          string  `json:"note,omitempty"`
}

func NewGetMedia(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {

	tool := mcp.NewTool("get_media",
		mcp.WithDescription("Get the photo or file attached to one message. A JPEG, PNG, WebP or GIF image up to 3.75 MB comes back as an image you can look at, after a JSON text block with message_id, media_key, kind, file_name, mime_type, size_bytes and image_returned. Any other file, or a larger image, returns only that JSON text block, with a note saying why the file is not included."),
		mcp.WithString("chat_id",
			mcp.Required(),
			mcp.Description("Chat ID (the chat's ref from list_chats)"),
		),
		mcp.WithNumber("message_id",
			mcp.Required(),
			mcp.Description("ID of the message that carries the media"),
		),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			ReadOnlyHint: boolPtr(true),
		}),
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

		info := mediaInfo{
			MessageID: messageID,
			MediaKey:  *media.Key,
			Kind:      kind,
			FileName:  media.FileName,
			MIMEType:  media.MIMEType,
			SizeBytes: media.FileSize,
		}
		if !looksLikeImage(media) {
			info.Note = "not an image; the file itself is not returned"
			return infoResult(info), nil
		}
		if media.FileSize != nil && *media.FileSize > imageMaxBytes {
			info.Note = fmt.Sprintf("image larger than %d bytes; not returned", imageMaxBytes)
			return infoResult(info), nil
		}

		data, err := c.GetMedia(ctx, chatID, *media.Key, imageMaxBytes)
		var tooLarge *client.TooLargeError
		if errors.As(err, &tooLarge) {
			info.Note = fmt.Sprintf("image larger than %d bytes; not returned", imageMaxBytes)
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
		info.ImageReturned = true
		return mcp.NewToolResultImage(infoText(info), base64.StdEncoding.EncodeToString(data), sniffed), nil
	}

	return tool, handler
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
	return mcp.NewToolResultText(infoText(info))
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
