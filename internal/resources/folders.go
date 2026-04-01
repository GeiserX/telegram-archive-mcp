package resources

import (
	"context"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterFolders wires telegram-archive://folders into the server.
func RegisterFolders(s *server.MCPServer, c *client.Client) {
	res := mcp.NewResource(
		"telegram-archive://folders",
		"Chat folders",
		mcp.WithResourceDescription("List of Telegram chat folders"),
		mcp.WithMIMEType("application/json"),
	)

	s.AddResource(res, func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		body, err := c.GetFolders()
		if err != nil {
			return nil, err
		}
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "telegram-archive://folders",
				MIMEType: "application/json",
				Text:     string(body),
			},
		}, nil
	})
}
