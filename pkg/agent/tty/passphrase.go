// Package tty provides terminal helpers shared by the agent subcommands: reading a
// passphrase without echo and asking a yes/no confirmation. Passphrases are read from
// standard input only through the explicit ReadPassphraseStdin helper; destructive
// confirmations stay interactive-only.
package tty

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

// ErrPassphraseMismatch is returned when the two entries during first-run
// passphrase creation do not match.
var ErrPassphraseMismatch = errors.New("passphrases do not match")

const (
	maxPassphraseBytes     = 4096
	maxPassphraseReadBytes = maxPassphraseBytes + len("\r\n") + 1
)

// ReadPassphrase reads a passphrase from the controlling terminal without echoing
// it. The prompt is written to stderr. It returns an error when stdin is not a
// terminal: the passphrase is never read from an environment variable or a pipe.
func ReadPassphrase(prompt string) ([]byte, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return nil, errors.New("a terminal is required to enter the agent passphrase")
	}
	fmt.Fprint(os.Stderr, prompt)
	pass, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		clear(pass)
		return nil, fmt.Errorf("read the passphrase: %w", err)
	}
	return pass, nil
}

// ReadPassphraseStdin reads one passphrase from r, for callers that have explicitly
// opted in to standard-input handling. It accepts EOF or one trailing LF (including
// CRLF), preserves all other bytes, and bounds input before allocating it in full.
func ReadPassphraseStdin(r io.Reader) ([]byte, error) {
	if r == nil {
		return nil, errors.New("read passphrase from standard input: input is unavailable")
	}
	// Read one byte beyond the largest valid value plus CRLF so oversized input
	// cannot be truncated into an apparently valid passphrase.
	pass := make([]byte, maxPassphraseReadBytes)
	n, err := io.ReadFull(r, pass)
	pass = pass[:n]
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		clear(pass)
		return nil, fmt.Errorf("read passphrase from standard input: %w", err)
	}
	if len(pass) > 0 && pass[len(pass)-1] == '\n' {
		pass[len(pass)-1] = 0
		pass = pass[:len(pass)-1]
		if len(pass) > 0 && pass[len(pass)-1] == '\r' {
			pass[len(pass)-1] = 0
			pass = pass[:len(pass)-1]
		}
	}
	if err := validatePassphrase(pass); err != nil {
		clear(pass)
		return nil, fmt.Errorf("read passphrase from standard input: %w", err)
	}
	return pass, nil
}

// PromptPassphrase prompts for the agent passphrase using read. When the key file
// does not yet exist (exists == false) it prompts twice and verifies the entries
// match, because the passphrase is the only way to ever decrypt tokens written
// under it. read is injected so callers (and tests) can supply their own reader.
func PromptPassphrase(read func(prompt string) ([]byte, error), exists bool) ([]byte, error) {
	if exists {
		pass, err := read("Enter the agent passphrase: ")
		if err != nil {
			clear(pass)
			return nil, err
		}
		if err := validatePassphrase(pass); err != nil {
			clear(pass)
			return nil, err
		}
		return pass, nil
	}
	pass, err := read("Enter a new agent passphrase: ")
	if err != nil {
		clear(pass)
		return nil, err
	}
	if err := validatePassphrase(pass); err != nil {
		clear(pass)
		return nil, err
	}
	confirm, err := read("Confirm the agent passphrase: ")
	if err != nil {
		clear(pass)
		clear(confirm)
		return nil, err
	}
	defer clear(confirm)
	if !bytes.Equal(pass, confirm) {
		clear(pass)
		return nil, ErrPassphraseMismatch
	}
	return pass, nil
}

func validatePassphrase(pass []byte) error {
	if len(pass) == 0 {
		return errors.New("passphrase is empty")
	}
	if len(pass) > maxPassphraseBytes {
		return fmt.Errorf("passphrase exceeds %d bytes", maxPassphraseBytes)
	}
	return nil
}
