package tools

import (
	"context"
	"math"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func NewListChats(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {

	tool := mcp.NewTool("list_chats",
		mcp.WithDescription("List archived Telegram chats, most recent first. Use this first: each chat's ref is the chat_id the other tools take. Each row also carries id (the numeric Telegram chat id), type, accounts (the archive account ids holding the chat), and its title or name. The answer is {chats, total, limit, offset, has_more}; page with offset while has_more is true."),
		mcp.WithNumber("limit",
			mcp.Description("Maximum chats to return (default 100, max 500)"),
		),
		mcp.WithNumber("offset",
			mcp.Description("Number of chats to skip (default 0)"),
		),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			ReadOnlyHint: boolPtr(true),
		}),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit := 100
		if v, ok := req.GetArguments()["limit"].(float64); ok && v > 0 {
			limit = int(v)
		}
		if limit > 500 {
			limit = 500
		}
		offset, _, err := wholeNumberArg(req.GetArguments(), "offset", math.MaxInt32)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		body, err := c.GetChats(ctx, limit, int(offset))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(string(body)), nil
	}

	return tool, handler
}
