package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/geiserx/telegram-archive-mcp/config"
	"github.com/geiserx/telegram-archive-mcp/internal/resources"
	"github.com/geiserx/telegram-archive-mcp/internal/tools"
	"github.com/geiserx/telegram-archive-mcp/version"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func isLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "" || host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// maxRequestBody caps one HTTP request body. MCP requests are small JSON
// documents; the SDK reads the whole body into memory, so without a cap one
// POST could exhaust it.
const maxRequestBody = 4 << 20 // 4 MiB

// bearerToken returns the token of an "Authorization: Bearer <token>" header.
// The scheme is matched case-insensitively (RFC 7235) and must be present: a
// bare token is refused.
func bearerToken(header string) (string, bool) {
	scheme, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimLeft(token, " ")
	if token == "" || strings.ContainsAny(token, " \t") {
		return "", false
	}
	return token, true
}

func bearerAuth(next http.Handler, token string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="telegram-archive-mcp"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// httpOptions configures the Streamable HTTP endpoint.
type httpOptions struct {
	// Token, when set, is required as a bearer token on every request.
	Token string
	// TrustedOrigins are browser origins (scheme://host[:port]) allowed to
	// call the endpoint cross-origin. Requests without an Origin header, as
	// every non-browser client sends, are unaffected.
	TrustedOrigins []string
}

// newHTTPHandler wraps the MCP endpoint in the protections the SDK does not
// apply itself: Origin validation (the spec's MUST, against DNS rebinding and
// cross-site requests), an optional bearer token and a request-body cap. The
// endpoint is stateless, so 2026-07-28 clients are served on it as well as
// legacy ones.
func newHTTPHandler(s *server.MCPServer, opts httpOptions) (http.Handler, *server.StreamableHTTPServer, error) {
	mcpHTTP := server.NewStreamableHTTPServer(s,
		server.WithEndpointPath("/mcp"),
		server.WithStateLess(true),
	)
	cop := http.NewCrossOriginProtection()
	for _, origin := range opts.TrustedOrigins {
		if err := cop.AddTrustedOrigin(origin); err != nil {
			return nil, nil, fmt.Errorf("MCP_ALLOWED_ORIGINS: %w", err)
		}
	}
	var h http.Handler = http.MaxBytesHandler(mcpHTTP, maxRequestBody)
	if opts.Token != "" {
		h = bearerAuth(h, opts.Token)
	}
	h = cop.Handler(h)

	mux := http.NewServeMux()
	mux.Handle("/mcp", h)
	return mux, mcpHTTP, nil
}

// newHTTPServer sets the timeouts a bare http.Server lacks, so a client that
// trickles headers or holds idle connections cannot pin the server. There is
// no WriteTimeout: a tool call that walks a whole day may legitimately take a
// while to answer.
func newHTTPServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}
}

// splitList splits a comma-separated environment value, dropping blanks.
func splitList(v string) []string {
	var out []string
	for _, part := range strings.Split(v, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// insecureArchiveURL reports whether the viewer URL sends the password and
// session cookie in cleartext across a network: plain http to a host that is
// not loopback, not a private address and not a single-label name such as a
// Docker Compose service.
func insecureArchiveURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" {
		return false
	}
	host := u.Hostname()
	if host == "localhost" || (!strings.Contains(host, ".") && net.ParseIP(host) == nil) {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return !ip.IsLoopback() && !ip.IsPrivate() && !ip.IsLinkLocalUnicast()
	}
	return true
}

// newServer builds the MCP server with every resource and tool registered.
func newServer(c *client.Client) *server.MCPServer {
	s := server.NewMCPServer(
		"Telegram-Archive MCP Bridge",
		version.Version,
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)

	resources.RegisterStats(s, c)
	resources.RegisterChats(s, c)
	resources.RegisterFolders(s, c)
	resources.RegisterHealth(s, c)

	for _, newTool := range []func(*client.Client) (mcp.Tool, server.ToolHandlerFunc){
		tools.NewListChats,
		tools.NewListFolders,
		tools.NewSearchMessages,
		tools.NewGetMessages,
		tools.NewGetPinnedMessages,
		tools.NewGetMessagesByDate,
		tools.NewGetChatStats,
		tools.NewGetTopics,
		tools.NewRefreshStats,
		tools.NewGetMedia,
	} {
		tool, handler := newTool(c)
		s.AddTool(tool, handler)
	}
	return s
}

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
			fmt.Fprintln(os.Stderr, "  MCP_AUTH_TOKEN         Bearer token for HTTP; required when LISTEN_ADDR is not loopback")
			fmt.Fprintln(os.Stderr, "  MCP_ALLOWED_ORIGINS    Comma-separated browser origins allowed to call /mcp (default: none)")
			os.Exit(0)
		case "--version", "-v":
			fmt.Fprintln(os.Stderr, version.String())
			os.Exit(0)
		}
	}

	log.Printf("Telegram-Archive MCP %s starting...", version.String())

	cfg := config.Load()
	if insecureArchiveURL(cfg.BaseURL) {
		log.Printf("warning: TELEGRAM_ARCHIVE_URL uses plain http to a public host; the viewer password and session cookie cross the network unencrypted")
	}
	c := client.New(cfg.BaseURL, cfg.User, cfg.Pass)

	s := newServer(c)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	transport := strings.ToLower(os.Getenv("TRANSPORT"))
	if transport == "stdio" {
		stdioSrv := server.NewStdioServer(s)
		log.Println("Telegram-Archive MCP bridge running on stdio")
		if err := stdioSrv.Listen(ctx, os.Stdin, os.Stdout); err != nil && ctx.Err() == nil {
			log.Fatalf("stdio server error: %v", err)
		}
		return
	}

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	authToken := os.Getenv("MCP_AUTH_TOKEN")
	if authToken == "" && !isLoopbackAddr(addr) {
		log.Fatal("MCP_AUTH_TOKEN is required when LISTEN_ADDR is not loopback")
	}
	handler, mcpHTTP, err := newHTTPHandler(s, httpOptions{
		Token:          authToken,
		TrustedOrigins: splitList(os.Getenv("MCP_ALLOWED_ORIGINS")),
	})
	if err != nil {
		log.Fatal(err)
	}
	srv := newHTTPServer(addr, handler)
	if authToken != "" {
		log.Printf("Telegram-Archive MCP bridge listening on %s (auth enabled)", addr)
	} else {
		log.Printf("Telegram-Archive MCP bridge listening on %s", addr)
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()

	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP shutdown error: %v", err)
	}
	if err := mcpHTTP.Shutdown(shutdownCtx); err != nil {
		log.Printf("MCP shutdown error: %v", err)
	}
}
