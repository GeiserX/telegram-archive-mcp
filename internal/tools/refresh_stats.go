package tools

import (
	"context"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func NewRefreshStats(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {

	tool := mcp.NewTool("refresh_stats",
		mcp.WithDescription("Make the viewer recalculate the archive-wide statistics now and return them (chat, message and media counts, total size). It scans the whole archive, so it can take a while and load the viewer's database; it changes no message. Needs the viewer's master login. The cached totals in telegram-archive://stats are usually enough. Hidden when the server runs with READ_ONLY=1."),
		describe("Recalculate archive statistics", false),
		mcp.WithRawOutputSchema([]byte(objectOutputSchema)),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		body, err := c.RefreshStats(ctx)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(body, ""), nil
	}

	return tool, handler
}
