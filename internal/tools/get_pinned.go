package tools

import (
	"context"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func NewGetPinnedMessages(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {

	tool := mcp.NewTool("get_pinned_messages",
		mcp.WithDescription("Get every pinned message of a Telegram chat, newest first, with the same fields as get_messages. Pinned messages often hold a group's rules, addresses or key links."),
		describe("Read pinned messages", true),
		chatIDParam(),
		mcp.WithRawOutputSchema([]byte(messagesOutputSchema)),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		chatID, err := req.RequireString("chat_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		body, err := c.GetPinnedMessages(ctx, chatID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(body, "messages"), nil
	}

	return tool, handler
}
