// internal/tools/get_topics.go
package tools

import (
	"context"
	"fmt"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NewGetTopics builds the tool definition and handler for retrieving
// topics in a chat (forum-style groups).
func NewGetTopics(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {

	tool := mcp.NewTool("get_topics",
		mcp.WithDescription("Get topics (forum threads) from a Telegram chat"),
		mcp.WithString("chat_id",
			mcp.Required(),
			mcp.Description("Chat ID to get topics from"),
		),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		chatID, err := req.RequireString("chat_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		body, err := c.GetTopics(chatID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(
			fmt.Sprintf("Topics: %s", string(body)),
		), nil
	}

	return tool, handler
}
