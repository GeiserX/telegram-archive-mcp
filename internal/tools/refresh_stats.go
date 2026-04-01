// internal/tools/refresh_stats.go
package tools

import (
	"context"
	"fmt"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NewRefreshStats builds the tool definition and handler for forcing
// a recalculation of global statistics.
func NewRefreshStats(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {

	tool := mcp.NewTool("refresh_stats",
		mcp.WithDescription("Force recalculation of global telegram-archive statistics"),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		body, err := c.RefreshStats(ctx)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(
			fmt.Sprintf("Stats refreshed. Response: %s", string(body)),
		), nil
	}

	return tool, handler
}
