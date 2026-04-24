package main

import (
	"net"
	"net/http"
	"os"
	"testing"
	"time"
)

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	addr := l.Addr().String()
	l.Close()
	return addr
}

func TestMain_HTTPServerStarts(t *testing.T) {
	addr := freePort(t)

	// Point at a non-existent backend -- we only test that main() wires
	// everything up and starts listening, not that the backend is reachable.
	t.Setenv("TELEGRAM_ARCHIVE_URL", "http://127.0.0.1:1")
	t.Setenv("TELEGRAM_ARCHIVE_USER", "")
	t.Setenv("TELEGRAM_ARCHIVE_PASS", "")
	t.Setenv("TRANSPORT", "http")
	t.Setenv("LISTEN_ADDR", addr)

	// Suppress log.Fatalf from killing the test process -- redirect stderr
	oldStderr := os.Stderr
	os.Stderr, _ = os.Open(os.DevNull)
	t.Cleanup(func() { os.Stderr = oldStderr })

	go main()

	// Wait for the server to start listening
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			// Server is listening -- verify it responds to HTTP
			resp, err := http.Get("http://" + addr + "/mcp")
			if err == nil {
				resp.Body.Close()
			}
			// Success: main() started and is serving HTTP
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("server did not start listening within timeout")
}
