package main

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/geiserx/telegram-archive-mcp/config"
	"github.com/geiserx/telegram-archive-mcp/internal/resources"
	"github.com/geiserx/telegram-archive-mcp/internal/tools"
	"github.com/geiserx/telegram-archive-mcp/version"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	log.Printf("Telegram-Archive MCP %s starting...", version.String())

	// Load config & initialise client
	cfg := config.Load()
	c := client.New(cfg.BaseURL, cfg.User, cfg.Pass)

	// Create MCP server
	s := server.NewMCPServer(
		"Telegram-Archive MCP Bridge",
		"0.0.1",
		server.WithToolCapabilities(true),
		server.WithRecovery(),
	)

	//----------------------------------------------------
	// Resources
	//----------------------------------------------------
	resources.RegisterStats(s, c)
	resources.RegisterChats(s, c)
	resources.RegisterFolders(s, c)
	resources.RegisterHealth(s, c)

	//----------------------------------------------------
	// Tools
	//----------------------------------------------------
	tool, handler := tools.NewSearchMessages(c)
	s.AddTool(tool, handler)

	tool, handler = tools.NewGetMessages(c)
	s.AddTool(tool, handler)

	tool, handler = tools.NewGetPinnedMessages(c)
	s.AddTool(tool, handler)

	tool, handler = tools.NewGetMessagesByDate(c)
	s.AddTool(tool, handler)

	tool, handler = tools.NewGetChatStats(c)
	s.AddTool(tool, handler)

	tool, handler = tools.NewGetTopics(c)
	s.AddTool(tool, handler)

	tool, handler = tools.NewRefreshStats(c)
	s.AddTool(tool, handler)

	transport := strings.ToLower(os.Getenv("TRANSPORT"))
	if transport == "stdio" {
		stdioSrv := server.NewStdioServer(s)
		log.Println("Telegram-Archive MCP bridge running on stdio")
		if err := stdioSrv.Listen(context.Background(), os.Stdin, os.Stdout); err != nil {
			log.Fatalf("stdio server error: %v", err)
		}
	} else {
		httpSrv := server.NewStreamableHTTPServer(s)
		log.Println("Telegram-Archive MCP bridge listening on :8080")
		if err := httpSrv.Start(":8080"); err != nil {
			log.Fatalf("server error: %v", err)
		}
	}
}
