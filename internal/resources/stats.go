package resources

import (
	"context"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterStats wires telegram-archive://stats into the server.
func RegisterStats(s *server.MCPServer, c *client.Client) {
	res := mcp.NewResource(
		"telegram-archive://stats",
		"Global backup statistics",
		mcp.WithResourceDescription("Chat count, message count, media count, total size"),
		mcp.WithMIMEType("application/json"),
	)

	s.AddResource(res, func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		body, err := c.GetStats(ctx)
		if err != nil {
			return nil, err
		}
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "telegram-archive://stats",
				MIMEType: "application/json",
				Text:     string(body),
			},
		}, nil
	})
}
