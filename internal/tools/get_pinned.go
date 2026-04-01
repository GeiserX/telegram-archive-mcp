// internal/tools/get_pinned.go
package tools

import (
	"context"
	"fmt"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NewGetPinnedMessages builds the tool definition and handler for retrieving
// pinned messages from a chat.
func NewGetPinnedMessages(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {

	tool := mcp.NewTool("get_pinned_messages",
		mcp.WithDescription("Get pinned messages from a Telegram chat"),
		mcp.WithString("chat_id",
			mcp.Required(),
			mcp.Description("Chat ID to get pinned messages from"),
		),
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

		return mcp.NewToolResultText(
			fmt.Sprintf("Pinned messages: %s", string(body)),
		), nil
	}

	return tool, handler
}
