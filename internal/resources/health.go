package resources

import (
	"context"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterHealth wires telegram-archive://health into the server.
func RegisterHealth(s *server.MCPServer, c *client.Client) {
	res := mcp.NewResource(
		"telegram-archive://health",
		"Health check",
		mcp.WithResourceDescription("Health status of the telegram-archive instance"),
		mcp.WithMIMEType("application/json"),
	)

	s.AddResource(res, func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		body, err := c.GetHealth()
		if err != nil {
			return nil, err
		}
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "telegram-archive://health",
				MIMEType: "application/json",
				Text:     string(body),
			},
		}, nil
	})
}
