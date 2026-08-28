package tty_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/suzuki-shunsuke/ghtkn/pkg/agent/tty"
)

const testMaxPassphraseBytes = 4096

func TestPromptPassphrase(t *testing.T) {
	t.Parallel()

	t.Run("existing prompts once", func(t *testing.T) {
		t.Parallel()
		calls := 0
		read := func(string) ([]byte, error) {
			calls++
			return []byte("pass"), nil
		}
		got, err := tty.PromptPassphrase(read, true)
		if err != nil {
			t.Fatal(err)
		}
		if calls != 1 {
			t.Fatalf("read called %d times, want 1", calls)
		}
		if string(got) != "pass" {
			t.Fatalf("passphrase = %q, want %q", got, "pass")
		}
	})

	t.Run("first run confirms", func(t *testing.T) {
		t.Parallel()
		calls := 0
		read := func(string) ([]byte, error) {
			calls++
			return []byte("pass"), nil
		}
		if _, err := tty.PromptPassphrase(read, false); err != nil {
			t.Fatal(err)
		}
		if calls != 2 {
			t.Fatalf("read called %d times, want 2", calls)
		}
	})

	t.Run("first run mismatch", func(t *testing.T) {
		t.Parallel()
		seq := [][]byte{[]byte("a"), []byte("b")}
		i := 0
		read := func(string) ([]byte, error) {
			v := seq[i]
			i++
			return v, nil
		}
		if _, err := tty.PromptPassphrase(read, false); !errors.Is(err, tty.ErrPassphraseMismatch) {
			t.Fatalf("err = %v, want tty.ErrPassphraseMismatch", err)
		}
	})
}

func TestPromptPassphraseValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		pass []byte
	}{
		{name: "empty"},
		{name: "unreasonable size", pass: []byte(strings.Repeat("x", testMaxPassphraseBytes+1))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := tty.PromptPassphrase(func(string) ([]byte, error) { return tt.pass, nil }, true); err == nil {
				t.Fatal("PromptPassphrase() succeeded with an invalid passphrase")
			}
		})
	}
}

func TestReadPassphraseStdin(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "EOF terminated", input: "passphrase", want: "passphrase"},
		{name: "LF terminated", input: "passphrase\n", want: "passphrase"},
		{name: "CRLF terminated", input: "passphrase\r\n", want: "passphrase"},
		{name: "embedded newline preserved", input: "pass\nphrase\n", want: "pass\nphrase"},
		{name: "only one trailing newline removed", input: "passphrase\n\n", want: "passphrase\n"},
		{name: "empty", wantErr: true},
		{name: "newline only", input: "\n", wantErr: true},
		{name: "maximum size", input: strings.Repeat("x", testMaxPassphraseBytes) + "\n", want: strings.Repeat("x", testMaxPassphraseBytes)},
		{name: "oversized", input: strings.Repeat("x", testMaxPassphraseBytes+1), wantErr: true},
		{name: "oversized after CRLF", input: strings.Repeat("x", testMaxPassphraseBytes) + "\r\nx", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := tty.ReadPassphraseStdin(strings.NewReader(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ReadPassphraseStdin() = %q, want an error", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Errorf("ReadPassphraseStdin() = %q, want %q", got, tt.want)
			}
		})
	}
}
