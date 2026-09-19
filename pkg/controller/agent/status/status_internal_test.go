package status

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestQueryStatus_notRunning(t *testing.T) {
	t.Parallel()
	resp, running, err := queryStatus(t.Context(), filepath.Join(t.TempDir(), "absent.sock"))
	if err != nil {
		t.Fatal(err)
	}
	if running {
		t.Fatal("queryStatus must report not running when the socket is absent")
	}
	if resp != nil {
		t.Fatalf("resp = %+v, want nil", resp)
	}
}

func TestCheck_notRunning(t *testing.T) {
	t.Setenv("GHTKN_AGENT_SOCKET", filepath.Join(os.TempDir(), "ghtkn-agent-check-absent.sock"))
	if err := New().Check(t.Context(), slog.New(slog.DiscardHandler)); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("Check with no agent running = %v, want ErrNotRunning", err)
	}
}

func TestRun_notRunningStillSucceeds(t *testing.T) {
	t.Setenv("GHTKN_AGENT_SOCKET", filepath.Join(os.TempDir(), "ghtkn-agent-status-absent.sock"))
	if err := New().Run(t.Context(), slog.New(slog.DiscardHandler)); err != nil {
		t.Fatalf("Run with no agent running = %v, want nil", err)
	}
}
