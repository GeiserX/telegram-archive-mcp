// internal/tools/get_messages.go
package tools

import (
	"context"
	"fmt"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NewGetMessages builds the tool definition and handler for retrieving
// messages from a chat with pagination.
func NewGetMessages(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {

	tool := mcp.NewTool("get_messages",
		mcp.WithDescription("Get messages from a Telegram chat with pagination"),
		mcp.WithString("chat_id",
			mcp.Required(),
			mcp.Description("Chat ID to retrieve messages from"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum messages to return (default 50)"),
		),
		mcp.WithNumber("offset",
			mcp.Description("Pagination offset (default 0)"),
		),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		chatID, err := req.RequireString("chat_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		limit := 50
		if v, ok := req.GetArguments()["limit"].(float64); ok && v > 0 {
			limit = int(v)
		}

		offset := 0
		if v, ok := req.GetArguments()["offset"].(float64); ok && v > 0 {
			offset = int(v)
		}

		body, err := c.GetMessages(chatID, limit, offset)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(
			fmt.Sprintf("Messages: %s", string(body)),
		), nil
	}

	return tool, handler
}
