package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/geiserx/telegram-archive-mcp/config"
	"github.com/geiserx/telegram-archive-mcp/internal/resources"
	"github.com/geiserx/telegram-archive-mcp/internal/tools"
	"github.com/geiserx/telegram-archive-mcp/version"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Handle --help / -h / --version gracefully so MCP client probes don't hang.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--help", "-h":
			fmt.Fprintf(os.Stderr, "Telegram-Archive MCP Bridge %s\n\n", version.String())
			fmt.Fprintln(os.Stderr, "Environment variables:")
			fmt.Fprintln(os.Stderr, "  TRANSPORT              stdio or http (default: http)")
			fmt.Fprintln(os.Stderr, "  LISTEN_ADDR            HTTP listen address (default: 127.0.0.1:8080)")
			fmt.Fprintln(os.Stderr, "  TELEGRAM_ARCHIVE_URL   Base URL of the Telegram Archive instance")
			fmt.Fprintln(os.Stderr, "  TELEGRAM_ARCHIVE_USER  Username for authentication")
			fmt.Fprintln(os.Stderr, "  TELEGRAM_ARCHIVE_PASS  Password for authentication")
			os.Exit(0)
		case "--version", "-v":
			fmt.Fprintln(os.Stderr, version.String())
			os.Exit(0)
		}
	}

	log.Printf("Telegram-Archive MCP %s starting...", version.String())

	cfg := config.Load()
	c := client.New(cfg.BaseURL, cfg.User, cfg.Pass)

	s := server.NewMCPServer(
		"Telegram-Archive MCP Bridge",
		version.Version,
		server.WithToolCapabilities(false),
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
	tool, handler := tools.NewListChats(c)
	s.AddTool(tool, handler)

	tool, handler = tools.NewListFolders(c)
	s.AddTool(tool, handler)

	tool, handler = tools.NewSearchMessages(c)
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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	transport := strings.ToLower(os.Getenv("TRANSPORT"))
	if transport == "stdio" {
		stdioSrv := server.NewStdioServer(s)
		log.Println("Telegram-Archive MCP bridge running on stdio")
		if err := stdioSrv.Listen(ctx, os.Stdin, os.Stdout); err != nil {
			log.Fatalf("stdio server error: %v", err)
		}
	} else {
		httpSrv := server.NewStreamableHTTPServer(s)
		addr := os.Getenv("LISTEN_ADDR")
		if addr == "" {
			addr = "127.0.0.1:8080"
		}
		log.Printf("Telegram-Archive MCP bridge listening on %s", addr)
		if err := httpSrv.Start(addr); err != nil {
			log.Fatalf("server error: %v", err)
		}
	}
}
