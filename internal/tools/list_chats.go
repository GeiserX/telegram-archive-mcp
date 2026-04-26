package tools

import (
	"context"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func NewListChats(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {

	tool := mcp.NewTool("list_chats",
		mcp.WithDescription("List all archived Telegram chats with their IDs, names, and types. Use this first to discover chat_id values needed by other tools."),
		mcp.WithNumber("limit",
			mcp.Description("Maximum chats to return (default 100)"),
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

		body, err := c.GetChats(ctx, limit)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(string(body)), nil
	}

	return tool, handler
}
