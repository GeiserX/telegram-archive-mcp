// internal/tools/get_by_date.go
package tools

import (
	"context"
	"fmt"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NewGetMessagesByDate builds the tool definition and handler for retrieving
// messages from a specific date.
func NewGetMessagesByDate(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {

	tool := mcp.NewTool("get_messages_by_date",
		mcp.WithDescription("Get messages from a Telegram chat on a specific date"),
		mcp.WithString("chat_id",
			mcp.Required(),
			mcp.Description("Chat ID to retrieve messages from"),
		),
		mcp.WithString("date",
			mcp.Required(),
			mcp.Description("Date in YYYY-MM-DD format"),
		),
		mcp.WithString("timezone",
			mcp.Description("IANA timezone (e.g. Europe/Madrid). Optional."),
		),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		chatID, err := req.RequireString("chat_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		date, err := req.RequireString("date")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		tz, _ := req.GetArguments()["timezone"].(string)

		body, err := c.GetMessagesByDate(chatID, date, tz)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(
			fmt.Sprintf("Messages by date: %s", string(body)),
		), nil
	}

	return tool, handler
}
