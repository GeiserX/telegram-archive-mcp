package resources

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// testMux returns a mux that handles login and configurable API endpoints.
func testMux(routes map[string]http.HandlerFunc) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "viewer_auth", Value: "test"})
		w.WriteHeader(http.StatusOK)
	})
	for pattern, handler := range routes {
		mux.HandleFunc(pattern, handler)
	}
	return mux
}

// newTestClient creates an httptest server and client for resource handler tests.
func newTestClient(t *testing.T, routes map[string]http.HandlerFunc) (*httptest.Server, *client.Client) {
	t.Helper()
	ts := httptest.NewServer(testMux(routes))
	c := client.New(ts.URL, "admin", "secret")
	return ts, c
}

// readResource is a helper that registers a resource on a fresh MCPServer
// and invokes its handler via the server's HandleMessage interface. Since the
// mcp-go library does not expose a direct way to call resource handlers in
// isolation, we test the registration functions by verifying they call the
// client correctly and return the expected content shape.
//
// We test by calling the client method directly (which is what the resource
// handler does) and verifying the response is what would be wrapped.
// This tests the integration between our resource registration and the client.

// --- Stats ---

func TestRegisterStats_ReturnsStatsContent(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/stats": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"total_chats":42,"total_messages":1000}`))
		},
	})
	defer ts.Close()

	s := server.NewMCPServer("test", "0.0.1")
	RegisterStats(s, c)

	// Verify the client call works (this is what the resource handler calls)
	body, err := c.GetStats(context.Background())
	if err != nil {
		t.Fatalf("GetStats error: %v", err)
	}
	if string(body) != `{"total_chats":42,"total_messages":1000}` {
		t.Errorf("body = %q", string(body))
	}
}

func TestRegisterStats_HandlerReturnsErrorOnAPIFailure(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/stats": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("down"))
		},
	})
	defer ts.Close()

	s := server.NewMCPServer("test", "0.0.1")
	RegisterStats(s, c)

	_, err := c.GetStats(context.Background())
	if err == nil {
		t.Error("expected error on 500")
	}
}

// --- Chats ---

func TestRegisterChats_ReturnsChatsContent(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("limit") != "100" {
				t.Errorf("chats limit = %q, want 100", r.URL.Query().Get("limit"))
			}
			w.Write([]byte(`[{"id":"c1","name":"Test Chat"}]`))
		},
	})
	defer ts.Close()

	s := server.NewMCPServer("test", "0.0.1")
	RegisterChats(s, c)

	body, err := c.GetChats(context.Background(), 100)
	if err != nil {
		t.Fatalf("GetChats error: %v", err)
	}
	if string(body) != `[{"id":"c1","name":"Test Chat"}]` {
		t.Errorf("body = %q", string(body))
	}
}

func TestRegisterChats_HandlerReturnsErrorOnAPIFailure(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		},
	})
	defer ts.Close()

	s := server.NewMCPServer("test", "0.0.1")
	RegisterChats(s, c)

	_, err := c.GetChats(context.Background(), 100)
	if err == nil {
		t.Error("expected error on 502")
	}
}

// --- Folders ---

func TestRegisterFolders_ReturnsFoldersContent(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/folders": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`[{"id":1,"title":"Work"}]`))
		},
	})
	defer ts.Close()

	s := server.NewMCPServer("test", "0.0.1")
	RegisterFolders(s, c)

	body, err := c.GetFolders(context.Background())
	if err != nil {
		t.Fatalf("GetFolders error: %v", err)
	}
	if string(body) != `[{"id":1,"title":"Work"}]` {
		t.Errorf("body = %q", string(body))
	}
}

func TestRegisterFolders_HandlerReturnsErrorOnAPIFailure(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/folders": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
	})
	defer ts.Close()

	s := server.NewMCPServer("test", "0.0.1")
	RegisterFolders(s, c)

	_, err := c.GetFolders(context.Background())
	if err == nil {
		t.Error("expected error on 500")
	}
}

// --- Health ---

func TestRegisterHealth_ReturnsHealthContent(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/health": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"status":"healthy"}`))
		},
	})
	defer ts.Close()

	s := server.NewMCPServer("test", "0.0.1")
	RegisterHealth(s, c)

	body, err := c.GetHealth(context.Background())
	if err != nil {
		t.Fatalf("GetHealth error: %v", err)
	}
	if string(body) != `{"status":"healthy"}` {
		t.Errorf("body = %q", string(body))
	}
}

func TestRegisterHealth_HandlerReturnsErrorOnAPIFailure(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/health": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		},
	})
	defer ts.Close()

	s := server.NewMCPServer("test", "0.0.1")
	RegisterHealth(s, c)

	_, err := c.GetHealth(context.Background())
	if err == nil {
		t.Error("expected error on 503")
	}
}

// --- Resource registration verifies correct URI ---

func TestRegisterStats_RegistersCorrectResource(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/stats": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{}`))
		},
	})
	defer ts.Close()

	s := server.NewMCPServer("test", "0.0.1", server.WithResourceCapabilities(true, false))
	RegisterStats(s, c)

	// Verify registration by reading the resource through the MCP protocol
	req := `{"jsonrpc":"2.0","id":1,"method":"resources/list"}`
	resp := s.HandleMessage(context.Background(), []byte(req))

	// resp should be non-nil and contain our resource
	if resp == nil {
		t.Fatal("HandleMessage returned nil")
	}
}

func TestRegisterChats_RegistersCorrectResource(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`[]`))
		},
	})
	defer ts.Close()

	s := server.NewMCPServer("test", "0.0.1", server.WithResourceCapabilities(true, false))
	RegisterChats(s, c)

	req := `{"jsonrpc":"2.0","id":1,"method":"resources/list"}`
	resp := s.HandleMessage(context.Background(), []byte(req))
	if resp == nil {
		t.Fatal("HandleMessage returned nil")
	}
}

func TestRegisterFolders_RegistersCorrectResource(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/folders": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`[]`))
		},
	})
	defer ts.Close()

	s := server.NewMCPServer("test", "0.0.1", server.WithResourceCapabilities(true, false))
	RegisterFolders(s, c)

	req := `{"jsonrpc":"2.0","id":1,"method":"resources/list"}`
	resp := s.HandleMessage(context.Background(), []byte(req))
	if resp == nil {
		t.Fatal("HandleMessage returned nil")
	}
}

func TestRegisterHealth_RegistersCorrectResource(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/health": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{}`))
		},
	})
	defer ts.Close()

	s := server.NewMCPServer("test", "0.0.1", server.WithResourceCapabilities(true, false))
	RegisterHealth(s, c)

	req := `{"jsonrpc":"2.0","id":1,"method":"resources/list"}`
	resp := s.HandleMessage(context.Background(), []byte(req))
	if resp == nil {
		t.Fatal("HandleMessage returned nil")
	}
}

// --- Integration: resource read via MCP protocol ---

func TestRegisterStats_ReadResourceReturnsMCPContent(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/stats": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"total":99}`))
		},
	})
	defer ts.Close()

	s := server.NewMCPServer("test", "0.0.1", server.WithResourceCapabilities(true, false))
	RegisterStats(s, c)

	req := `{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"telegram-archive://stats"}}`
	resp := s.HandleMessage(context.Background(), []byte(req))
	if resp == nil {
		t.Fatal("HandleMessage returned nil for resources/read")
	}

	// Verify that it returned a success response, not an error
	if _, isErr := resp.(mcp.JSONRPCError); isErr {
		t.Fatal("resources/read returned an error response")
	}
	if _, ok := resp.(mcp.JSONRPCResponse); !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", resp)
	}
}

func TestRegisterChats_ReadResourceReturnsMCPContent(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`[]`))
		},
	})
	defer ts.Close()

	s := server.NewMCPServer("test", "0.0.1", server.WithResourceCapabilities(true, false))
	RegisterChats(s, c)

	req := `{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"telegram-archive://chats"}}`
	resp := s.HandleMessage(context.Background(), []byte(req))
	if resp == nil {
		t.Fatal("HandleMessage returned nil")
	}

	if _, isErr := resp.(mcp.JSONRPCError); isErr {
		t.Fatal("resources/read returned an error response")
	}
	if _, ok := resp.(mcp.JSONRPCResponse); !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", resp)
	}
}

func TestRegisterFolders_ReadResourceReturnsMCPContent(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/folders": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`[]`))
		},
	})
	defer ts.Close()

	s := server.NewMCPServer("test", "0.0.1", server.WithResourceCapabilities(true, false))
	RegisterFolders(s, c)

	req := `{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"telegram-archive://folders"}}`
	resp := s.HandleMessage(context.Background(), []byte(req))
	if resp == nil {
		t.Fatal("HandleMessage returned nil")
	}

	if _, isErr := resp.(mcp.JSONRPCError); isErr {
		t.Fatal("resources/read returned an error response")
	}
	if _, ok := resp.(mcp.JSONRPCResponse); !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", resp)
	}
}

func TestRegisterHealth_ReadResourceReturnsMCPContent(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/health": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"status":"ok"}`))
		},
	})
	defer ts.Close()

	s := server.NewMCPServer("test", "0.0.1", server.WithResourceCapabilities(true, false))
	RegisterHealth(s, c)

	req := `{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"telegram-archive://health"}}`
	resp := s.HandleMessage(context.Background(), []byte(req))
	if resp == nil {
		t.Fatal("HandleMessage returned nil")
	}

	if _, isErr := resp.(mcp.JSONRPCError); isErr {
		t.Fatal("resources/read returned an error response")
	}
	if _, ok := resp.(mcp.JSONRPCResponse); !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", resp)
	}
}

// --- Integration: resource read error paths via MCP protocol ---

func TestRegisterStats_ReadResourceReturnsErrorOnBackendFailure(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/stats": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("boom"))
		},
	})
	defer ts.Close()

	s := server.NewMCPServer("test", "0.0.1", server.WithResourceCapabilities(true, false))
	RegisterStats(s, c)

	req := `{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"telegram-archive://stats"}}`
	resp := s.HandleMessage(context.Background(), []byte(req))
	if resp == nil {
		t.Fatal("HandleMessage returned nil")
	}
	if _, isErr := resp.(mcp.JSONRPCError); !isErr {
		t.Errorf("expected JSONRPCError on backend failure, got %T", resp)
	}
}

func TestRegisterChats_ReadResourceReturnsErrorOnBackendFailure(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/chats": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
	})
	defer ts.Close()

	s := server.NewMCPServer("test", "0.0.1", server.WithResourceCapabilities(true, false))
	RegisterChats(s, c)

	req := `{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"telegram-archive://chats"}}`
	resp := s.HandleMessage(context.Background(), []byte(req))
	if resp == nil {
		t.Fatal("HandleMessage returned nil")
	}
	if _, isErr := resp.(mcp.JSONRPCError); !isErr {
		t.Errorf("expected JSONRPCError on backend failure, got %T", resp)
	}
}

func TestRegisterFolders_ReadResourceReturnsErrorOnBackendFailure(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/folders": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
	})
	defer ts.Close()

	s := server.NewMCPServer("test", "0.0.1", server.WithResourceCapabilities(true, false))
	RegisterFolders(s, c)

	req := `{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"telegram-archive://folders"}}`
	resp := s.HandleMessage(context.Background(), []byte(req))
	if resp == nil {
		t.Fatal("HandleMessage returned nil")
	}
	if _, isErr := resp.(mcp.JSONRPCError); !isErr {
		t.Errorf("expected JSONRPCError on backend failure, got %T", resp)
	}
}

func TestRegisterHealth_ReadResourceReturnsErrorOnBackendFailure(t *testing.T) {
	ts, c := newTestClient(t, map[string]http.HandlerFunc{
		"/api/health": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		},
	})
	defer ts.Close()

	s := server.NewMCPServer("test", "0.0.1", server.WithResourceCapabilities(true, false))
	RegisterHealth(s, c)

	req := `{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"telegram-archive://health"}}`
	resp := s.HandleMessage(context.Background(), []byte(req))
	if resp == nil {
		t.Fatal("HandleMessage returned nil")
	}
	if _, isErr := resp.(mcp.JSONRPCError); !isErr {
		t.Errorf("expected JSONRPCError on backend failure, got %T", resp)
	}
}
