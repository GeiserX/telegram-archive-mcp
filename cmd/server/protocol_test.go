package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"slices"
	"testing"
	"time"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/server"
)

// modernMeta is the per-request _meta a 2026-07-28 client sends instead of an
// initialize handshake.
var modernMeta = map[string]any{
	"io.modelcontextprotocol/protocolVersion":    "2026-07-28",
	"io.modelcontextprotocol/clientCapabilities": map[string]any{},
	"io.modelcontextprotocol/clientInfo":         map[string]any{"name": "smoke", "version": "0"},
}

// stdioPeer drives the real server over an in-process stdio pipe, the way a
// client that spawned the binary would.
type stdioPeer struct {
	t   *testing.T
	in  *io.PipeWriter
	out *bufio.Reader
}

func newStdioPeer(t *testing.T) *stdioPeer {
	t.Helper()
	s := newServer(client.New("http://127.0.0.1:1", "", ""))
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = server.NewStdioServer(s).Listen(ctx, inR, outW)
		outW.Close()
	}()
	t.Cleanup(func() {
		cancel()
		inW.Close()
		<-done
	})
	return &stdioPeer{t: t, in: inW, out: bufio.NewReader(outR)}
}

type rpcReply struct {
	ID     json.RawMessage `json:"id"`
	Result map[string]any  `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// call sends one request and returns the reply that carries its id.
func (p *stdioPeer) call(id int, method string, params map[string]any) rpcReply {
	p.t.Helper()
	p.send(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		line, err := p.out.ReadBytes('\n')
		if err != nil {
			p.t.Fatalf("%s: reading reply: %v", method, err)
		}
		var r rpcReply
		if err := json.Unmarshal(line, &r); err != nil {
			p.t.Fatalf("%s: reply is not JSON: %s", method, line)
		}
		if string(r.ID) == jsonInt(id) {
			return r
		}
	}
	p.t.Fatalf("%s: no reply with id %d", method, id)
	return rpcReply{}
}

func (p *stdioPeer) send(msg map[string]any) {
	p.t.Helper()
	b, err := json.Marshal(msg)
	if err != nil {
		p.t.Fatal(err)
	}
	if _, err := p.in.Write(append(b, '\n')); err != nil {
		p.t.Fatalf("write: %v", err)
	}
}

func jsonInt(i int) string {
	b, _ := json.Marshal(i)
	return string(b)
}

func toolNames(t *testing.T, result map[string]any) []string {
	t.Helper()
	raw, ok := result["tools"].([]any)
	if !ok || len(raw) == 0 {
		t.Fatalf("result has no tools: %v", result)
	}
	var names []string
	for _, x := range raw {
		names = append(names, x.(map[string]any)["name"].(string))
	}
	return names
}

func TestProtocol_ModernDiscover(t *testing.T) {
	p := newStdioPeer(t)
	r := p.call(1, "server/discover", map[string]any{"_meta": modernMeta})
	if r.Error != nil {
		t.Fatalf("server/discover failed: %d %s", r.Error.Code, r.Error.Message)
	}
	raw, _ := r.Result["supportedVersions"].([]any)
	var versions []string
	for _, v := range raw {
		versions = append(versions, v.(string))
	}
	for _, want := range []string{"2026-07-28", "2025-11-25", "2024-11-05"} {
		if !slices.Contains(versions, want) {
			t.Errorf("supportedVersions = %v, missing %s", versions, want)
		}
	}
}

func TestProtocol_ModernToolsListIsCacheable(t *testing.T) {
	p := newStdioPeer(t)
	r := p.call(1, "tools/list", map[string]any{"_meta": modernMeta})
	if r.Error != nil {
		t.Fatalf("tools/list failed: %d %s", r.Error.Code, r.Error.Message)
	}
	if r.Result["resultType"] != "complete" {
		t.Errorf("resultType = %v, want complete", r.Result["resultType"])
	}
	if _, ok := r.Result["ttlMs"].(float64); !ok {
		t.Errorf("ttlMs missing: %v", r.Result)
	}
	if _, ok := r.Result["cacheScope"].(string); !ok {
		t.Errorf("cacheScope missing: %v", r.Result)
	}
	if names := toolNames(t, r.Result); !slices.Contains(names, "list_chats") {
		t.Errorf("tools = %v, missing list_chats", names)
	}
}

func TestProtocol_ModernPingIsRemoved(t *testing.T) {
	p := newStdioPeer(t)
	r := p.call(1, "ping", map[string]any{"_meta": modernMeta})
	if r.Error == nil || r.Error.Code != -32601 {
		t.Fatalf("modern ping = %+v, want error -32601", r)
	}
}

func TestProtocol_LegacyInitializeStillWorks(t *testing.T) {
	p := newStdioPeer(t)
	r := p.call(1, "initialize", map[string]any{
		"protocolVersion": "2025-11-25",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "legacy", "version": "0"},
	})
	if r.Error != nil {
		t.Fatalf("initialize failed: %d %s", r.Error.Code, r.Error.Message)
	}
	if r.Result["protocolVersion"] != "2025-11-25" {
		t.Errorf("protocolVersion = %v, want 2025-11-25", r.Result["protocolVersion"])
	}
	p.send(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"})
	r = p.call(2, "tools/list", map[string]any{})
	if r.Error != nil {
		t.Fatalf("legacy tools/list failed: %d %s", r.Error.Code, r.Error.Message)
	}
	if names := toolNames(t, r.Result); !slices.Contains(names, "get_messages") {
		t.Errorf("tools = %v, missing get_messages", names)
	}
	if r := p.call(3, "ping", map[string]any{}); r.Error != nil {
		t.Errorf("legacy ping failed: %d %s", r.Error.Code, r.Error.Message)
	}
}
