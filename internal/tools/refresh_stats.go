package tools

import (
	"context"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func NewRefreshStats(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {

	tool := mcp.NewTool("refresh_stats",
		mcp.WithDescription("Force recalculation of global telegram-archive statistics"),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			DestructiveHint: boolPtr(false),
			IdempotentHint:  boolPtr(true),
		}),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		body, err := c.RefreshStats(ctx)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(string(body)), nil
	}

	return tool, handler
}
