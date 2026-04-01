// internal/tools/get_chat_stats.go
package tools

import (
	"context"
	"fmt"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NewGetChatStats builds the tool definition and handler for retrieving
// statistics for a specific chat.
func NewGetChatStats(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {

	tool := mcp.NewTool("get_chat_stats",
		mcp.WithDescription("Get statistics for a specific Telegram chat"),
		mcp.WithString("chat_id",
			mcp.Required(),
			mcp.Description("Chat ID to get statistics for"),
		),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		chatID, err := req.RequireString("chat_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		body, err := c.GetChatStats(chatID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(
			fmt.Sprintf("Chat stats: %s", string(body)),
		), nil
	}

	return tool, handler
}
