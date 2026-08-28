package agent

import (
	"strings"
	"testing"

	"github.com/suzuki-shunsuke/ghtkn/pkg/cli/flag"
)

func TestUnlockCommand_passphraseStdinFlag(t *testing.T) {
	t.Parallel()
	cmd := (&runner{flags: &flag.GlobalFlags{}}).unlockCommand()
	if cmd.Flags().Lookup("passphrase-stdin") == nil {
		t.Fatal("unlock command does not define --passphrase-stdin")
	}
	if !strings.Contains(cmd.Long, "never read without this explicit flag") {
		t.Fatal("unlock command help does not document the explicit stdin opt-in")
	}
	if !strings.Contains(cmd.Long, "op read --no-newline op://Private/ghtkn-passphrase/password | ghtkn agent unlock --enable-refresh --passphrase-stdin") {
		t.Fatal("unlock command help does not show the supported 1Password pipeline")
	}
}
