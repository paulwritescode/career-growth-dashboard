package bridge

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// readUntil reads frames until one contains want, or the deadline passes.
func readUntil(t *testing.T, conn *websocket.Conn, want string) string {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read (waiting for %q): %v", want, err)
		}
		if strings.Contains(string(msg), want) {
			return string(msg)
		}
	}
}

// TestBridgeTurnRelay uses `echo` as a stand-in for kiro-cli: the bridge passes
// the user's message as the prompt argument, so echo prints it straight back —
// proving a turn round-trips browser -> process -> browser.
func TestBridgeTurnRelay(t *testing.T) {
	b := newBridge("echo", nil, nil, false, nil, quietLogger())
	srv := httptest.NewServer(http.HandlerFunc(b.HandleWS))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteMessage(websocket.TextMessage, []byte("hello-bridge")); err != nil {
		t.Fatalf("write: %v", err)
	}
	frame := readUntil(t, conn, "hello-bridge")
	if !strings.Contains(frame, `"type":"chunk"`) {
		t.Fatalf("expected a chunk frame carrying the echo, got %q", frame)
	}
}

func TestBridgeBadBinary(t *testing.T) {
	b := New("definitely-not-a-real-binary-xyz", nil, false, nil, quietLogger())
	srv := httptest.NewServer(http.HandlerFunc(b.HandleWS))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// A turn must be requested before the bridge tries to launch the binary.
	if err := conn.WriteMessage(websocket.TextMessage, []byte("hi")); err != nil {
		t.Fatalf("write: %v", err)
	}
	readUntil(t, conn, "could not start kiro-cli")
}

func TestCheckOrigin(t *testing.T) {
	b := New("cat", []string{"127.0.0.1:5500", "localhost:5500"}, false, nil, quietLogger())

	cases := []struct {
		origin string
		want   bool
	}{
		{"", true}, // non-browser client
		{"http://127.0.0.1:5500", true},
		{"http://localhost:5500", true},
		{"http://evil.example.com", false},
		{"http://127.0.0.1:9999", false},
	}
	for _, c := range cases {
		r := httptest.NewRequest("GET", "/ws", nil)
		if c.origin != "" {
			r.Header.Set("Origin", c.origin)
		}
		if got := b.checkOrigin(r); got != c.want {
			t.Errorf("checkOrigin(%q) = %v, want %v", c.origin, got, c.want)
		}
	}
}

func TestValidHost(t *testing.T) {
	b := New("cat", []string{"127.0.0.1:5500"}, false, nil, quietLogger())
	good := []string{"127.0.0.1:5500", "localhost:5500", "127.0.0.1:1234", "[::1]:5500"}
	bad := []string{"evil.example.com", "192.168.1.5:5500", "10.0.0.1:80"}
	for _, h := range good {
		if !b.validHost(h) {
			t.Errorf("validHost(%q) = false, want true", h)
		}
	}
	for _, h := range bad {
		if b.validHost(h) {
			t.Errorf("validHost(%q) = true, want false", h)
		}
	}
}

func TestCleanLineStripsANSI(t *testing.T) {
	cases := map[string]string{
		"\x1b[38;5;11mWARNING\x1b[0m":                      "WARNING",
		"\x1b[?25l\x1b[38;5;141m> \x1b[0mBRIDGE_OK\x1b[0m": "BRIDGE_OK",
		"plain text":    "plain text",
		"> prefixed":    "prefixed",
		"  trailing   ": "trailing",
	}
	for in, want := range cases {
		if got := cleanLine(in); got != want {
			t.Errorf("cleanLine(%q) = %q, want %q", in, got, want)
		}
	}
}

// ensure url parsing helper assumptions hold (guards against import pruning).
var _ = url.Parse
