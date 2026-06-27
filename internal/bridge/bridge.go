// Package bridge connects the dashboard chat drawer to a local kiro-cli process
// so the agent is reachable from the desktop dashboard. Each browser message is
// run as a one-shot `kiro-cli chat --no-interactive "<message>"` turn (with
// --resume for continuity), the output is stripped of terminal escape codes,
// and the result is streamed back over the WebSocket as JSON frames.
//
// Why one-shot turns rather than relaying the interactive TUI: `kiro-cli chat`
// in its default mode is a full-screen terminal UI that emits ANSI/cursor
// control sequences and expects a TTY — unusable over a plain WebSocket. The
// --no-interactive mode takes the prompt as an argument and prints a plain
// answer, which is what a web client can render.
//
// Security (specs/10): the agent is launched with an explicit args slice (never
// a shell string), Origin and Host are validated against a loopback allowlist
// to defend against cross-site WebSocket hijacking / DNS-rebinding, and tools
// are left untrusted unless the operator explicitly opts in (--kiro-trust-all).
package bridge

import (
	"bufio"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	maxMessageBytes = 1 << 20 // 1 MiB inbound cap
	maxLineBytes    = 8 << 20 // 8 MiB stdout line cap
	writeWait       = 10 * time.Second
	turnTimeout     = 5 * time.Minute // hard cap on a single agent turn
)

// ansiRE matches ANSI/VT100 escape sequences (CSI, OSC, and single-char escapes)
// so the agent's colored/cursor-control output renders as clean text in the UI.
var ansiRE = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b[@-Z\\-_]`)

// IntentRunner executes an allowlisted mutation intent emitted by the agent.
// It is satisfied by *service.Service. Defining it here (rather than importing
// service) keeps the bridge a thin transport that only knows how to relay and
// dispatch, not the business rules behind each intent.
type IntentRunner interface {
	RunChatIntent(ctx context.Context, name string, fields map[string]string) (string, error)
}

// intentPreamble is prepended to each user turn so kiro-cli knows how to ask
// the dashboard to record data. The agent emits a single SCAVA-ACTION line that
// the bridge parses and routes through the service layer; anything else is
// relayed as ordinary chat (specs/07 mechanism 1).
const intentPreamble = `[local-scava] You are wired into the user's career dashboard and can record entries for them. If — and ONLY if — the user is clearly asking to create or change a tracked entry, reply with ONE line and nothing else:
SCAVA-ACTION {"intent":"NAME","fields":{"key":"value"}}
Allowed NAME and fields:
- log.record: worked_on, insight, next_up, what_happened, blocker
- post.create: post_type(daily|recap), title
- post.draft: tier(blog|linkedin|x), content
- post.mark_published: tier(blog|linkedin|x), url
- sprint.create: skill_name, microapp_one_liner, core_feature, skill_rationale, out_of_scope, deploy_platform
- sprint.set_phase: phase(1-4)
- sprint.ship: live_url
- adr.create: title, problem, decision, why, consequences
Use only the listed fields; never invent intents. For anything else (questions, drafting help, chit-chat) answer normally with no SCAVA-ACTION line. The user's message follows:
`

// Bridge holds the configuration for spawning and proxying kiro-cli sessions.
type Bridge struct {
	kiroBin      string
	baseArgs     []string
	trustAll     bool
	runner       IntentRunner
	allowedHosts map[string]struct{}
	log          *slog.Logger
	upgrader     websocket.Upgrader
}

// frame is a single message sent to the browser. Type is one of:
//   - "system":  status/error lines (rendered muted)
//   - "chunk":   a line of agent output
//   - "action":  a confirmation that a mutation was performed
//   - "refresh": a hint for the UI to reload the affected page content
//   - "done":    the current turn has finished (re-enable input)
type frame struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// New builds a Bridge that drives kiro-cli in non-interactive mode. allowedHosts
// is the set of acceptable Host/Origin hosts (e.g. "127.0.0.1:5500",
// "localhost:5500"). When trustAll is true the agent may run tools without
// confirmation (passes -a) — a deliberate, riskier opt-in. runner executes
// allowlisted mutation intents; pass nil to disable chat-driven mutations.
func New(kiroBin string, allowedHosts []string, trustAll bool, runner IntentRunner, log *slog.Logger) *Bridge {
	return newBridge(kiroBin, []string{"chat", "--no-interactive"}, allowedHosts, trustAll, runner, log)
}

// newBridge is the shared constructor (baseArgs are injectable for tests).
func newBridge(kiroBin string, baseArgs, allowedHosts []string, trustAll bool, runner IntentRunner, log *slog.Logger) *Bridge {
	hosts := make(map[string]struct{}, len(allowedHosts))
	for _, h := range allowedHosts {
		hosts[h] = struct{}{}
	}
	b := &Bridge{
		kiroBin:      kiroBin,
		baseArgs:     baseArgs,
		trustAll:     trustAll,
		runner:       runner,
		allowedHosts: hosts,
		log:          log,
	}
	b.upgrader = websocket.Upgrader{
		HandshakeTimeout: 10 * time.Second,
		CheckOrigin:      b.checkOrigin,
	}
	return b
}

// checkOrigin permits same-origin/non-browser clients (no Origin header) and
// rejects any Origin whose host is not in the allowlist. This is the primary
// defense against cross-site WebSocket hijacking.
func (b *Bridge) checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true // non-browser client (CLI/test); loopback bind still protects
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	_, ok := b.allowedHosts[u.Host]
	return ok
}

// validHost guards against DNS-rebinding: the Host header must be loopback or
// an explicitly allowed host.
func (b *Bridge) validHost(host string) bool {
	if _, ok := b.allowedHosts[host]; ok {
		return true
	}
	h := host
	if i := strings.LastIndex(host, ":"); i >= 0 {
		h = host[:i]
	}
	h = strings.Trim(h, "[]")
	return h == "127.0.0.1" || h == "localhost" || h == "::1" || strings.HasPrefix(h, "127.")
}

// HandleWS upgrades the connection and runs a chat loop: each inbound browser
// message becomes one non-interactive kiro-cli turn whose output streams back.
func (b *Bridge) HandleWS(w http.ResponseWriter, r *http.Request) {
	if !b.validHost(r.Host) {
		http.Error(w, "forbidden host", http.StatusForbidden)
		return
	}
	conn, err := b.upgrader.Upgrade(w, r, nil)
	if err != nil {
		b.log.Warn("ws upgrade failed", "err", err)
		return
	}
	defer conn.Close()
	conn.SetReadLimit(maxMessageBytes)

	var writeMu sync.Mutex
	b.writeFrame(conn, &writeMu, frame{Type: "system", Text: "connected to kiro-cli — ask me to log today, start a sprint, or draft a post"})
	b.log.Info("bridge session started", "kiro", b.kiroBin)
	defer b.log.Info("bridge session ended")

	resume := false
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		text := strings.TrimSpace(string(msg))
		if text == "" {
			continue
		}
		if err := b.runTurn(r.Context(), conn, &writeMu, text, resume); err != nil {
			b.writeFrame(conn, &writeMu, frame{Type: "system", Text: "kiro-cli error: " + err.Error()})
		} else {
			resume = true // subsequent turns continue the same conversation
		}
		b.writeFrame(conn, &writeMu, frame{Type: "done"})
	}
}

// runTurn executes a single non-interactive kiro-cli turn and streams its
// cleaned stdout back to the browser as chunk frames.
func (b *Bridge) runTurn(parent context.Context, conn *websocket.Conn, mu *sync.Mutex, text string, resume bool) error {
	ctx, cancel := context.WithTimeout(parent, turnTimeout)
	defer cancel()

	args := append([]string{}, b.baseArgs...)
	if resume {
		args = append(args, "--resume")
	}
	if b.trustAll {
		args = append(args, "-a")
	}
	// Prepend the intent protocol so the agent can record entries. Only when an
	// intent runner is wired; otherwise the chat is purely conversational.
	prompt := text
	if b.runner != nil {
		prompt = intentPreamble + text
	}
	// "--" terminates flag parsing so a message starting with "-" is treated as
	// the prompt, not a flag.
	args = append(args, "--", prompt)

	cmd := exec.CommandContext(ctx, b.kiroBin, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = nil // warnings on stderr are noise for the chat panel

	if err := cmd.Start(); err != nil {
		return &startError{bin: b.kiroBin, err: err}
	}
	b.log.Debug("bridge turn started", "pid", cmd.Process.Pid, "resume", resume)

	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 64*1024), maxLineBytes)
	var (
		capturing bool
		pending   string
	)
	for sc.Scan() {
		line := cleanLine(sc.Text())
		// Detect and capture a SCAVA-ACTION directive (possibly spanning lines).
		if b.runner != nil && !capturing {
			if i := strings.Index(line, "SCAVA-ACTION"); i >= 0 {
				capturing = true
				pending = line[i+len("SCAVA-ACTION"):]
				if js, ok := extractJSON(pending); ok {
					b.dispatchIntent(ctx, conn, mu, js)
					capturing, pending = false, ""
				}
				continue
			}
		} else if capturing {
			pending += "\n" + line
			if js, ok := extractJSON(pending); ok {
				b.dispatchIntent(ctx, conn, mu, js)
				capturing, pending = false, ""
			}
			continue
		}
		if line == "" {
			continue
		}
		if err := b.writeFrame(conn, mu, frame{Type: "chunk", Text: line}); err != nil {
			cancel()
			break
		}
	}
	return cmd.Wait()
}

// dispatchIntent parses a captured SCAVA-ACTION JSON payload, runs it through
// the allowlisted intent runner, and emits an action confirmation + a refresh
// hint. Validation/allowlist failures are surfaced as system text, never as a
// silent execution.
func (b *Bridge) dispatchIntent(ctx context.Context, conn *websocket.Conn, mu *sync.Mutex, jsonStr string) {
	var raw struct {
		Intent string         `json:"intent"`
		Fields map[string]any `json:"fields"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		b.writeFrame(conn, mu, frame{Type: "system", Text: "could not parse action: " + err.Error()})
		return
	}
	fields := make(map[string]string, len(raw.Fields))
	for k, v := range raw.Fields {
		fields[k] = coerceString(v)
	}
	summary, err := b.runner.RunChatIntent(ctx, raw.Intent, fields)
	if err != nil {
		b.writeFrame(conn, mu, frame{Type: "system", Text: "action failed: " + err.Error()})
		return
	}
	b.log.Info("chat intent executed", "intent", raw.Intent, "summary", summary)
	b.writeFrame(conn, mu, frame{Type: "action", Text: summary})
	b.writeFrame(conn, mu, frame{Type: "refresh"})
}

// extractJSON returns the first balanced {...} object found in s.
func extractJSON(s string) (string, bool) {
	i := strings.IndexByte(s, '{')
	if i < 0 {
		return "", false
	}
	depth := 0
	for j := i; j < len(s); j++ {
		switch s[j] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[i : j+1], true
			}
		}
	}
	return "", false
}

// coerceString renders a JSON value as a plain string for intent fields.
func coerceString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	case nil:
		return ""
	default:
		return ""
	}
}

// startError reports a failure to launch kiro-cli with a stable, testable prefix.
type startError struct {
	bin string
	err error
}

func (e *startError) Error() string {
	return "could not start kiro-cli (" + e.bin + "): " + e.err.Error()
}

// cleanLine strips ANSI escape codes, a leading kiro prompt glyph, and trailing
// whitespace from a line of agent output.
func cleanLine(s string) string {
	s = ansiRE.ReplaceAllString(s, "")
	s = strings.TrimRight(s, " \t\r")
	s = strings.TrimPrefix(s, "> ")
	return strings.TrimSpace(s)
}

// writeFrame marshals and sends a frame to the browser under the write lock.
func (b *Bridge) writeFrame(conn *websocket.Conn, mu *sync.Mutex, f frame) error {
	data, err := json.Marshal(f)
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
	return conn.WriteMessage(websocket.TextMessage, data)
}
