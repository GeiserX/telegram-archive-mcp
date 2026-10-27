package tools

import (
	"context"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func NewGetChatStats(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {

	tool := mcp.NewTool("get_chat_stats",
		mcp.WithDescription("Get the archive's statistics for one chat: how many messages and media files it holds, their total size, and the dates of its first and last message. Use telegram-archive://stats for the totals of the whole archive."),
		describe("Chat statistics", true),
		chatIDParam(),
		mcp.WithRawOutputSchema([]byte(objectOutputSchema)),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		chatID, err := req.RequireString("chat_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		body, err := c.GetChatStats(ctx, chatID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(body, ""), nil
	}

	return tool, handler
}
