// internal/tools/search_messages.go
package tools

import (
	"context"
	"fmt"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NewSearchMessages builds the tool definition and handler for searching
// messages inside a specific chat.
func NewSearchMessages(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {

	tool := mcp.NewTool("search_messages",
		mcp.WithDescription("Search messages in a Telegram chat by keyword"),
		mcp.WithString("chat_id",
			mcp.Required(),
			mcp.Description("Chat ID to search in"),
		),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Search query string"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum results to return (default 20)"),
		),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		chatID, err := req.RequireString("chat_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		query, err := req.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		limit := 20
		if v, ok := req.GetArguments()["limit"].(float64); ok && v > 0 {
			limit = int(v)
		}
		if limit > 200 {
			limit = 200
		}

		body, err := c.SearchMessages(ctx, chatID, query, limit)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(
			fmt.Sprintf("Search results: %s", string(body)),
		), nil
	}

	return tool, handler
}
