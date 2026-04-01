package resources

import (
	"context"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterChats wires telegram-archive://chats into the server.
func RegisterChats(s *server.MCPServer, c *client.Client) {
	res := mcp.NewResource(
		"telegram-archive://chats",
		"Chat list",
		mcp.WithResourceDescription("List of all archived chats with basic info"),
		mcp.WithMIMEType("application/json"),
	)

	s.AddResource(res, func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		body, err := c.GetChats(100)
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
