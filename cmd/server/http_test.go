package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/geiserx/telegram-archive-mcp/client"
)

func newTestHTTP(t *testing.T, opts httpOptions) *httptest.Server {
	t.Helper()
	s := newServer(client.New("http://127.0.0.1:1", "", ""), serverOptions{})
	h, _, err := newHTTPHandler(s, opts)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts
}

func modernToolsList() []byte {
	b, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/list",
		"params": map[string]any{"_meta": modernMeta},
	})
	return b
}

// post sends one JSON-RPC request; headers are added on top of the ones every
// client sends.
func post(t *testing.T, url string, body []byte, headers map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range headers {
		if k == "Host" {
			req.Host = v
			continue
		}
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

var modernHeaders = map[string]string{
	"MCP-Protocol-Version": "2026-07-28",
	"Mcp-Method":           "tools/list",
}

// decodeReply reads a JSON-RPC reply sent as plain JSON or as one SSE event.
func decodeReply(t *testing.T, resp *http.Response) rpcReply {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		sc := bufio.NewScanner(bytes.NewReader(body))
		for sc.Scan() {
			if data, ok := strings.CutPrefix(sc.Text(), "data: "); ok {
				body = []byte(data)
				break
			}
		}
	}
	var r rpcReply
	if err := json.Unmarshal(body, &r); err != nil {
		t.Fatalf("reply is not JSON-RPC: %s", body)
	}
	return r
}

func TestHTTP_ModernToolsListOnStatelessEndpoint(t *testing.T) {
	ts := newTestHTTP(t, httpOptions{})
	resp := post(t, ts.URL+"/mcp", modernToolsList(), modernHeaders)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if id := resp.Header.Get("Mcp-Session-Id"); id != "" {
		t.Errorf("stateless endpoint minted session id %q", id)
	}
	r := decodeReply(t, resp)
	if r.Error != nil || r.Result["resultType"] != "complete" {
		t.Fatalf("reply = %+v, want a complete result", r)
	}
}

func TestHTTP_LegacyInitializeThenToolsList(t *testing.T) {
	ts := newTestHTTP(t, httpOptions{})
	init, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-11-25",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "legacy", "version": "0"},
		},
	})
	r := decodeReply(t, post(t, ts.URL+"/mcp", init, nil))
	if r.Error != nil || r.Result["protocolVersion"] != "2025-11-25" {
		t.Fatalf("initialize = %+v", r)
	}
	list, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/list"})
	r = decodeReply(t, post(t, ts.URL+"/mcp", list, map[string]string{"MCP-Protocol-Version": "2025-11-25"}))
	if r.Error != nil {
		t.Fatalf("legacy tools/list failed: %d %s", r.Error.Code, r.Error.Message)
	}
	toolNames(t, r.Result)
}

func TestHTTP_RejectsCrossSiteOrigin(t *testing.T) {
	ts := newTestHTTP(t, httpOptions{})
	headers := map[string]string{"Origin": "https://evil.example", "Sec-Fetch-Site": "cross-site"}
	for k, v := range modernHeaders {
		headers[k] = v
	}
	if resp := post(t, ts.URL+"/mcp", modernToolsList(), headers); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-site POST: status = %d, want 403", resp.StatusCode)
	}
	// Origin without Sec-Fetch-Site (an older browser): still refused, the
	// origin does not match the host.
	delete(headers, "Sec-Fetch-Site")
	if resp := post(t, ts.URL+"/mcp", modernToolsList(), headers); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("foreign Origin: status = %d, want 403", resp.StatusCode)
	}
}

// A DNS-rebinding page and its request share one host name, so the Origin
// check passes them. The SDK refuses a non-localhost Host on a loopback
// connection; this keeps that default from being switched off.
func TestHTTP_RefusesARebindingHost(t *testing.T) {
	ts := newTestHTTP(t, httpOptions{})
	port := ts.URL[strings.LastIndex(ts.URL, ":"):]
	headers := map[string]string{"Host": "evil.example" + port, "Origin": "http://evil.example" + port, "Sec-Fetch-Site": "same-origin"}
	for k, v := range modernHeaders {
		headers[k] = v
	}
	if resp := post(t, ts.URL+"/mcp", modernToolsList(), headers); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("rebinding Host: status = %d, want 403", resp.StatusCode)
	}
	headers["Host"], headers["Origin"] = "localhost"+port, "http://localhost"+port
	if resp := post(t, ts.URL+"/mcp", modernToolsList(), headers); resp.StatusCode != http.StatusOK {
		t.Fatalf("localhost Host: status = %d, want 200", resp.StatusCode)
	}
}

func TestHTTP_TrustedOriginIsAllowed(t *testing.T) {
	ts := newTestHTTP(t, httpOptions{TrustedOrigins: []string{"https://inspector.example"}})
	headers := map[string]string{"Origin": "https://inspector.example", "Sec-Fetch-Site": "cross-site"}
	for k, v := range modernHeaders {
		headers[k] = v
	}
	if resp := post(t, ts.URL+"/mcp", modernToolsList(), headers); resp.StatusCode != http.StatusOK {
		t.Fatalf("trusted origin: status = %d, want 200", resp.StatusCode)
	}
}

func TestHTTP_RejectsBadTrustedOrigin(t *testing.T) {
	s := newServer(client.New("http://127.0.0.1:1", "", ""), serverOptions{})
	if _, _, err := newHTTPHandler(s, httpOptions{TrustedOrigins: []string{"not a url"}}); err == nil {
		t.Fatal("want an error for a malformed MCP_ALLOWED_ORIGINS entry")
	}
}

func TestHTTP_CapsRequestBody(t *testing.T) {
	ts := newTestHTTP(t, httpOptions{})
	// A valid request padded to n bytes of filler: only the size differs.
	padded := func(n int) []byte {
		return bytes.Replace(modernToolsList(), []byte(`"params":{`),
			append([]byte(`"params":{"pad":"`+strings.Repeat("x", n)), []byte(`",`)...), 1)
	}
	// Just under the cap: served.
	r := decodeReply(t, post(t, ts.URL+"/mcp", padded(maxRequestBody-4096), modernHeaders))
	if r.Error != nil {
		t.Fatalf("a body under the cap was refused: %d %s", r.Error.Code, r.Error.Message)
	}
	// Over it: 400 with a "too large" parse error, before any tool runs.
	resp := post(t, ts.URL+"/mcp", padded(maxRequestBody+1), modernHeaders)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("over the cap: status = %d, want 400", resp.StatusCode)
	}
	if r := decodeReply(t, resp); r.Error == nil || !strings.Contains(r.Error.Message, "too large") {
		t.Fatalf("over the cap: reply = %+v, want a body-too-large error", r)
	}
}

func TestHTTP_BearerAuth(t *testing.T) {
	const token = "s3cret-token"
	ts := newTestHTTP(t, httpOptions{Token: token})
	cases := []struct {
		header string
		want   int
	}{
		{"", http.StatusUnauthorized},
		{token, http.StatusUnauthorized}, // no scheme
		{"Bearer wrong", http.StatusUnauthorized},
		{"Basic " + token, http.StatusUnauthorized},
		{"Bearer " + token + " extra", http.StatusUnauthorized},
		{"Bearer " + token, http.StatusOK},
		{"bearer " + token, http.StatusOK}, // RFC 7235: the scheme is case-insensitive
	}
	for _, tc := range cases {
		headers := map[string]string{}
		for k, v := range modernHeaders {
			headers[k] = v
		}
		if tc.header != "" {
			headers["Authorization"] = tc.header
		}
		resp := post(t, ts.URL+"/mcp", modernToolsList(), headers)
		if resp.StatusCode != tc.want {
			t.Errorf("Authorization %q: status = %d, want %d", tc.header, resp.StatusCode, tc.want)
		}
		if tc.want == http.StatusUnauthorized && resp.Header.Get("WWW-Authenticate") == "" {
			t.Errorf("Authorization %q: 401 without WWW-Authenticate", tc.header)
		}
	}
}

func TestIsLoopbackAddr(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1:8080": true,
		"[::1]:8080":     true,
		"localhost:8080": true,
		":8080":          false, // binds every interface
		"LOCALHOST:8080": true,
		"0.0.0.0:8080":   false,
		"10.0.0.5:8080":  false,
		"example.com:80": false,
		"no-port":        false,
	}
	for addr, want := range cases {
		if got := isLoopbackAddr(addr); got != want {
			t.Errorf("isLoopbackAddr(%q) = %v, want %v", addr, got, want)
		}
	}
}

func TestInsecureArchiveURL(t *testing.T) {
	cases := map[string]bool{
		"http://localhost:8000":         false,
		"http://127.0.0.1:8000":         false,
		"http://telegram-viewer:8000":   false, // Docker Compose service name
		"http://192.168.1.10:8000":      false,
		"https://archive.example.com":   false,
		"http://archive.example.com":    true,
		"http://203.0.113.7:8000":       true,
		"http://[2001:db8::1]:8000/api": true,
	}
	for raw, want := range cases {
		if got := insecureArchiveURL(raw); got != want {
			t.Errorf("insecureArchiveURL(%q) = %v, want %v", raw, got, want)
		}
	}
}
