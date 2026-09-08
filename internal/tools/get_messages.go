package tools

import (
	"context"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func NewGetMessages(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {

	tool := mcp.NewTool("get_messages",
		mcp.WithDescription("Get messages from a Telegram chat, newest first. Page with offset, or with the keyset cursor: pass the date and id of the last message you received as before_date and before_id to get the older ones (constant time on huge chats). after_id returns messages newer than an id. Dates are naive UTC."),
		mcp.WithString("chat_id",
			mcp.Required(),
			mcp.Description("Chat ID to retrieve messages from"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum messages to return (default 50, max 500)"),
		),
		mcp.WithNumber("offset",
			mcp.Description("Pagination offset (default 0). Ignored when a cursor is given."),
		),
		mcp.WithString("before_date",
			mcp.Description("Keyset cursor: ISO 8601 date-time, naive values are UTC (e.g. 2026-06-10T18:04:17). Returns messages older than this instant; pair with before_id."),
		),
		mcp.WithNumber("before_id",
			mcp.Description("Keyset cursor: message id. Returns messages with a smaller id; pair with before_date."),
		),
		mcp.WithNumber("after_id",
			mcp.Description("Returns messages with an id greater than this one (newer)."),
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
		args := req.GetArguments()

		limit := 50
		if v, ok := args["limit"].(float64); ok && v > 0 {
			limit = int(v)
		}
		if limit > 500 {
			limit = 500
		}

		var cur client.MessagesCursor
		if v, ok := args["before_date"].(string); ok && v != "" {
			t, err := parseArchiveTime(v)
			if err != nil {
				return mcp.NewToolResultError("invalid before_date: " + err.Error()), nil
			}
			cur.BeforeDate = formatArchiveTime(t)
		}
		if v, ok := args["before_id"].(float64); ok && v > 0 {
			cur.BeforeID = int64(v)
		}
		if v, ok := args["after_id"].(float64); ok && v > 0 {
			cur.AfterID = int64(v)
		}

		if cur != (client.MessagesCursor{}) {
			body, err := c.GetMessagesCursor(ctx, chatID, limit, cur)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(body)), nil
		}

		offset := 0
		if v, ok := args["offset"].(float64); ok && v > 0 {
			offset = int(v)
		}
		if offset > 100000 {
			offset = 100000
		}

		body, err := c.GetMessages(ctx, chatID, limit, offset)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(string(body)), nil
	}

	return tool, handler
}
