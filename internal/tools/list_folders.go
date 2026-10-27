package tools

import (
	"context"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const listFoldersOutputSchema = `{"type":"object","properties":{"folders":{"type":"array","description":"One row per Telegram folder (dialog filter), including its id (the folder_id list_chats takes) and title.","items":{"type":"object"}}}}`

func NewListFolders(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {

	tool := mcp.NewTool("list_folders",
		mcp.WithDescription("List the Telegram chat folders (Telegram's dialog filters, such as Work or Family) with their ids. Pass a folder's id to list_chats as folder_id to see the chats inside it."),
		describe("List chat folders", true),
		mcp.WithRawOutputSchema([]byte(listFoldersOutputSchema)),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		body, err := c.GetFolders(ctx)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(body, "folders"), nil
	}

	return tool, handler
}
