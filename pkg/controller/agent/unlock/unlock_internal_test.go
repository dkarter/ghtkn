package unlock

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	agentapi "github.com/suzuki-shunsuke/ghtkn-go-sdk/ghtkn/backend/agent"
)

type countingReader struct {
	reader io.Reader
	reads  int
}

func (r *countingReader) Read(p []byte) (int, error) {
	r.reads++
	return r.reader.Read(p) //nolint:wrapcheck // Preserve the wrapped reader's io.Reader contract in this test helper.
}

// serveAgent starts a Unix-socket server that answers each request with handler, and
// returns a getEnv stub that points GHTKN_AGENT_SOCKET at it. Injecting the socket path
// through the Controller's getEnv (instead of t.Setenv) keeps the tests parallel-safe.
func serveAgent(t *testing.T, handler func(*agentapi.Request) *agentapi.Response) func(string) string {
	t.Helper()
	// A short dir keeps the socket path under the OS sun_path limit (t.TempDir embeds
	// the long test name).
	dir, err := os.MkdirTemp("", "gh") //nolint:usetesting // t.TempDir's path is too long for a unix socket
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	socket := filepath.Join(dir, "s.sock")

	lc := net.ListenConfig{}
	ln, err := lc.Listen(t.Context(), "unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			serveConn(conn, handler)
		}
	}()

	return func(k string) string {
		if k == "GHTKN_AGENT_SOCKET" {
			return socket
		}
		return ""
	}
}

// serveConn reads one newline-delimited request, answers it with handler, and writes the
// response back.
func serveConn(conn net.Conn, handler func(*agentapi.Request) *agentapi.Response) {
	defer conn.Close()
	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		return
	}
	req := &agentapi.Request{}
	if err := json.Unmarshal(line, req); err != nil {
		return
	}
	b, err := json.Marshal(handler(req))
	if err != nil {
		return
	}
	_, _ = conn.Write(append(b, '\n'))
}

// pendingHandler answers STATUS with locked, the first UNLOCK with RefreshTokenRemovalPending,
// and any UNLOCK carrying ConfirmRefreshTokenRemoval with OK. It records every UNLOCK seen.
func pendingHandler(mu *sync.Mutex, unlocks *[]*agentapi.Request) func(*agentapi.Request) *agentapi.Response {
	return func(req *agentapi.Request) *agentapi.Response {
		switch req.Command {
		case agentapi.CommandStatus:
			return &agentapi.Response{OK: true, Locked: true, Initialized: true}
		case agentapi.CommandUnlock:
			mu.Lock()
			*unlocks = append(*unlocks, req)
			mu.Unlock()
			if req.ConfirmRefreshTokenRemoval {
				return &agentapi.Response{OK: true}
			}
			return &agentapi.Response{RefreshTokenRemovalPending: true, Error: "confirm"}
		default:
			return &agentapi.Response{Error: "unexpected command"}
		}
	}
}

// TestController_Run_enableRefresh verifies that --enable-refresh reaches the wire: the
// UNLOCK request the client sends carries EnableRefreshToken.
func TestController_Run_enableRefresh(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var unlockReq *agentapi.Request
	getEnv := serveAgent(t, func(req *agentapi.Request) *agentapi.Response {
		switch req.Command {
		case agentapi.CommandStatus:
			return &agentapi.Response{OK: true, Locked: true, Initialized: true}
		case agentapi.CommandUnlock:
			mu.Lock()
			unlockReq = req
			mu.Unlock()
			return &agentapi.Response{OK: true, RefreshTokenEnabled: req.EnableRefreshToken}
		default:
			return &agentapi.Response{Error: "unexpected command"}
		}
	})

	c := &Controller{
		readPassphrase: func(string) ([]byte, error) { return []byte("pw"), nil },
		getEnv:         getEnv,
	}
	if err := c.Run(t.Context(), slog.New(slog.DiscardHandler), nil, false, true, 0); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	if unlockReq == nil {
		t.Fatal("no UNLOCK request was received")
	}
	if !unlockReq.EnableRefreshToken {
		t.Fatal("the UNLOCK request must carry EnableRefreshToken=true")
	}
}

// TestController_Run_confirmRefreshRemoval verifies that when the agent reports
// RefreshTokenRemovalPending and the user confirms, the client re-sends the unlock with
// ConfirmRefreshTokenRemoval set.
func TestController_Run_confirmRefreshRemoval(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var unlocks []*agentapi.Request
	getEnv := serveAgent(t, pendingHandler(&mu, &unlocks))

	c := &Controller{
		readPassphrase: func(string) ([]byte, error) { return []byte("pw"), nil },
		confirm:        func(string) (bool, error) { return true, nil },
		getEnv:         getEnv,
	}
	if err := c.Run(t.Context(), slog.New(slog.DiscardHandler), nil, false, false, 0); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(unlocks) != 2 {
		t.Fatalf("expected 2 UNLOCK requests (initial + confirmed), got %d", len(unlocks))
	}
	if unlocks[0].ConfirmRefreshTokenRemoval {
		t.Fatal("the first UNLOCK must not carry the confirmation")
	}
	if !unlocks[1].ConfirmRefreshTokenRemoval {
		t.Fatal("the re-sent UNLOCK must carry ConfirmRefreshTokenRemoval=true")
	}
}

// TestController_Run_declineRefreshRemoval verifies that declining the prompt aborts
// without re-sending the unlock and without an error (the agent stays locked).
func TestController_Run_declineRefreshRemoval(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var unlocks []*agentapi.Request
	getEnv := serveAgent(t, pendingHandler(&mu, &unlocks))

	c := &Controller{
		readPassphrase: func(string) ([]byte, error) { return []byte("pw"), nil },
		confirm:        func(string) (bool, error) { return false, nil },
		getEnv:         getEnv,
	}
	if err := c.Run(t.Context(), slog.New(slog.DiscardHandler), nil, false, false, 0); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(unlocks) != 1 {
		t.Fatalf("expected only the initial UNLOCK after declining, got %d", len(unlocks))
	}
}

func TestController_Run_passphraseInput(t *testing.T) { //nolint:cyclop,funlen // The table covers both initialization states and both input modes.
	t.Parallel()
	tests := []struct {
		name            string
		initialized     bool
		passphraseStdin bool
		interactive     [][]byte
		wantReads       int
	}{
		{
			name:        "interactive existing key prompts once",
			initialized: true,
			interactive: [][]byte{[]byte("interactive")},
			wantReads:   1,
		},
		{
			name:        "interactive first-time key prompts twice",
			interactive: [][]byte{[]byte("interactive"), []byte("interactive")},
			wantReads:   2,
		},
		{
			name:            "stdin existing key does not prompt",
			initialized:     true,
			passphraseStdin: true,
		},
		{
			name:            "stdin first-time key consumes one value",
			passphraseStdin: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var mu sync.Mutex
			var gotPassphrase []byte
			getEnv := serveAgent(t, func(req *agentapi.Request) *agentapi.Response {
				switch req.Command {
				case agentapi.CommandStatus:
					return &agentapi.Response{OK: true, Locked: true, Initialized: tt.initialized}
				case agentapi.CommandUnlock:
					mu.Lock()
					gotPassphrase = append([]byte(nil), req.Passphrase...)
					mu.Unlock()
					return &agentapi.Response{OK: true}
				default:
					return &agentapi.Response{Error: "unexpected command"}
				}
			})

			reads := 0
			c := &Controller{
				readPassphrase: func(string) ([]byte, error) {
					if reads >= len(tt.interactive) {
						return nil, errors.New("unexpected interactive passphrase read")
					}
					pass := tt.interactive[reads]
					reads++
					return append([]byte(nil), pass...), nil
				},
				getEnv: getEnv,
			}
			stdin := &countingReader{reader: strings.NewReader("piped\n")}
			if err := c.Run(t.Context(), slog.New(slog.DiscardHandler), stdin, tt.passphraseStdin, false, 0); err != nil {
				t.Fatal(err)
			}
			if reads != tt.wantReads {
				t.Errorf("interactive reads = %d, want %d", reads, tt.wantReads)
			}
			if !tt.passphraseStdin && stdin.reads != 0 {
				t.Errorf("standard input reads = %d, want 0 without --passphrase-stdin", stdin.reads)
			}
			if tt.passphraseStdin && stdin.reads == 0 {
				t.Error("standard input was not read with --passphrase-stdin")
			}
			want := "interactive"
			if tt.passphraseStdin {
				want = "piped"
			}
			mu.Lock()
			defer mu.Unlock()
			if string(gotPassphrase) != want {
				t.Errorf("wire passphrase = %q, want %q", gotPassphrase, want)
			}
		})
	}
}
