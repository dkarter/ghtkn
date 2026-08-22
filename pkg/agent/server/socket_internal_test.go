//go:build !windows

package server

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func shortSocketPath(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp("/tmp", "ghtkn-*.sock")
	if err != nil {
		t.Fatal(err)
	}
	path := f.Name()
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	return path
}

func createStaleSocket(t *testing.T, path string) os.FileInfo {
	t.Helper()
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	listener.SetUnlinkOnClose(false)
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info
}

func TestCleanupStaleSocket(t *testing.T) {
	t.Parallel()

	t.Run("no file", func(t *testing.T) {
		t.Parallel()
		if err := cleanupStaleSocket(t.Context(), filepath.Join(t.TempDir(), "absent.sock")); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("stale socket removed", func(t *testing.T) {
		t.Parallel()
		path := shortSocketPath(t)
		createStaleSocket(t, path)
		if err := cleanupStaleSocket(t.Context(), path); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("stale socket file was not removed: err=%v", err)
		}
	})

	t.Run("regular file preserved", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "agent.sock")
		if err := os.WriteFile(path, []byte("keep"), socketFilePerm); err != nil {
			t.Fatal(err)
		}
		if err := cleanupStaleSocket(t.Context(), path); err == nil {
			t.Fatal("cleanupStaleSocket must reject a regular file")
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("regular file was removed: %v", err)
		}
		if string(content) != "keep" {
			t.Fatalf("regular file was changed: %q", content)
		}
	})

	t.Run("symlink preserved", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		target := filepath.Join(dir, "target")
		path := filepath.Join(dir, "agent.sock")
		if err := os.WriteFile(target, []byte("keep"), socketFilePerm); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, path); err != nil {
			t.Fatal(err)
		}
		if err := cleanupStaleSocket(t.Context(), path); err == nil {
			t.Fatal("cleanupStaleSocket must reject a symlink")
		}
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatalf("symlink was removed: %v", err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("socket path is no longer a symlink: mode=%v", info.Mode())
		}
	})
}

func TestRemoveStaleSocketIfUnchangedPreservesReplacement(t *testing.T) {
	t.Parallel()
	path := shortSocketPath(t)
	info := createStaleSocket(t, path)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("keep"), socketFilePerm); err != nil {
		t.Fatal(err)
	}

	if err := removeStaleSocketIfUnchanged(path, info); err == nil {
		t.Fatal("removeStaleSocketIfUnchanged must reject a replacement file")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("replacement file was removed: %v", err)
	}
	if string(content) != "keep" {
		t.Fatalf("replacement file was changed: %q", content)
	}
}
