package tools

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func boolPtr(b bool) *bool { return &b }

// describe sets the tool's title and all four behaviour hints explicitly, so
// no client falls back on the spec defaults (destructive, open world). Every
// tool reads a closed, self-hosted archive, so none is open world; the only
// writer, refresh_stats, recomputes statistics and destroys nothing.
func describe(title string, readOnly bool) mcp.ToolOption {
	return func(t *mcp.Tool) {
		mcp.WithToolTitle(title)(t)
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:           title,
			ReadOnlyHint:    boolPtr(readOnly),
			DestructiveHint: boolPtr(false),
			IdempotentHint:  boolPtr(true),
			OpenWorldHint:   boolPtr(false),
		})(t)
	}
}

// chatIDParam is the chat_id every per-chat tool takes.
func chatIDParam() mcp.ToolOption {
	return mcp.WithString("chat_id",
		mcp.Required(),
		mcp.MinLength(1),
		mcp.Description("The chat's ref, as list_chats or search_all_chats returns it (an opaque string; not the numeric Telegram id)."),
	)
}

// topicIDParam filters a forum supergroup to one topic.
func topicIDParam() mcp.ToolOption {
	return mcp.WithInteger("topic_id",
		mcp.Min(1),
		mcp.Description("Forum topic id from get_topics; keeps only that topic's messages. 1 is the General topic. Omit for the whole chat."),
	)
}

// topicIDArg reads the optional topic_id argument.
func topicIDArg(args map[string]any) (int64, error) {
	v, present, err := wholeNumberArg(args, "topic_id", 1<<53-1)
	if err != nil {
		return 0, err
	}
	if present && v < 1 {
		return 0, errors.New("topic_id must be at least 1")
	}
	return v, nil
}

// limitArg reads an optional limit: absent or 0 gives def, larger than max
// is capped at max, and anything that is not a whole number is an error.
// Through the server a limit above max never gets here: the tool's input
// schema declares the same maximum and refuses the call first. The cap only
// guards a handler called directly, as the unit tests do.
func limitArg(args map[string]any, def, max int64) (int64, error) {
	v, present, err := wholeNumberArg(args, "limit", math.MaxInt32)
	if err != nil {
		return 0, err
	}
	if !present || v == 0 {
		return def, nil
	}
	return min(v, max), nil
}

// Output schemas. They describe the envelope the tool returns and leave the
// rows open: the rows are the viewer's own JSON, whose fields grow with its
// versions.
const (
	messagesOutputSchema = `{"type":"object","properties":{"messages":{"type":"array","description":"Archived messages as the viewer returns them: id, date (naive UTC), text, sender fields, reply_to_msg_id, media and more.","items":{"type":"object"}}}}`
	objectOutputSchema   = `{"type":"object"}`
)

// jsonResult returns the viewer's JSON both as text, unchanged for clients
// that read only content, and as structuredContent. A JSON array is wrapped as
// {arrayKey: [...]} because structuredContent must be an object for clients
// before 2026-07-28. Numbers are kept as written, so 64-bit ids survive.
func jsonResult(body []byte, arrayKey string) *mcp.CallToolResult {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return mcp.NewToolResultError("unexpected answer from the archive: not JSON")
	}
	switch x := v.(type) {
	case map[string]any:
		return mcp.NewToolResultStructured(x, string(body))
	case []any:
		if arrayKey != "" {
			return mcp.NewToolResultStructured(map[string]any{arrayKey: x}, string(body))
		}
	}
	return mcp.NewToolResultError(fmt.Sprintf("unexpected answer from the archive: a JSON %T", v))
}

// All returns every tool, in the order they are registered. A tool whose
// readOnlyHint is false changes something in the viewer.
func All(c *client.Client) []server.ServerTool {
	var out []server.ServerTool
	for _, newTool := range []func(*client.Client) (mcp.Tool, server.ToolHandlerFunc){
		NewListChats,
		NewListFolders,
		NewSearchAllChats,
		NewSearchMessages,
		NewGetMessages,
		NewGetMessageContext,
		NewGetPinnedMessages,
		NewGetMessagesByDate,
		NewGetChatStats,
		NewGetTopics,
		NewGetMedia,
		NewRefreshStats,
	} {
		tool, handler := newTool(c)
		out = append(out, server.ServerTool{Tool: tool, Handler: handler})
	}
	return out
}

// IsReadOnly reports whether a tool only reads.
func IsReadOnly(t mcp.Tool) bool {
	return t.Annotations.ReadOnlyHint != nil && *t.Annotations.ReadOnlyHint
}
