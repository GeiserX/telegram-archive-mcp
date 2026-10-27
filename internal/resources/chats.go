package resources

import (
	"context"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// chatsResourceLimit is the largest page list_chats allows.
const chatsResourceLimit = 500

// RegisterChats wires telegram-archive://chats into the server.
func RegisterChats(s *server.MCPServer, c *client.Client) {
	res := mcp.NewResource(
		"telegram-archive://chats",
		"Chat list",
		mcp.WithResourceDescription("The 500 most recent archived chats with basic info, as {chats, total, limit, offset, has_more}; list_chats pages further and filters by name."),
		mcp.WithMIMEType("application/json"),
	)

	s.AddResource(res, func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		// Without a limit the viewer returns its default page of 50.
		body, err := c.GetChats(ctx, chatsResourceLimit, 0)
		if err != nil {
			return nil, err
		}
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "telegram-archive://chats",
				MIMEType: "application/json",
				Text:     string(body),
			},
		}, nil
	})
}
