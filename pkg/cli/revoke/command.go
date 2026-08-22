// Package revoke implements the 'ghtkn revoke' command.
// It revokes GitHub App User Access Tokens via GitHub's credential revocation API
// and removes the revoked tokens from the backend. This is useful when a token has
// been leaked and must be revoked quickly.
//
// Raw tokens should be supplied through standard input with --token-stdin so they do
// not appear in process arguments or shell history. Positional raw tokens remain
// supported for compatibility. Other positional arguments are treated as app names,
// whose stored tokens are revoked and removed from the backend. When no argument or
// stdin token is given, the token stored for GHTKN_APP (or the default app) is
// revoked.
//
// The --all flag revokes the stored tokens of every app in the config at once,
// for incident response when the environment running ghtkn is compromised. With
// --all, app name arguments are ignored, but raw access tokens are still revoked.
package revoke

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
	"github.com/suzuki-shunsuke/ghtkn/pkg/cli/completion"
	"github.com/suzuki-shunsuke/ghtkn/pkg/cli/flag"
	"github.com/suzuki-shunsuke/ghtkn/pkg/config"
	"github.com/suzuki-shunsuke/ghtkn/pkg/controller/revoke"
	"github.com/suzuki-shunsuke/slog-util/slogutil"
)

// Args holds the flag and argument values for the revoke command.
type Args struct {
	*flag.GlobalFlags

	Args        []string  // positional arguments (legacy raw tokens and/or app names)
	TokenReader io.Reader // standard input when --token-stdin is set
	All         bool      // --all: revoke the stored tokens of every app in the config
	TokenStdin  bool      // --token-stdin: read one raw token from standard input
}

const (
	initialTokenStdinBufferBytes = 1024
	maxTokenStdinBytes           = 4096
)

// New creates a new revoke command instance with the provided logger.
// It returns a CLI command that can be registered with the main CLI application.
func New(logger *slogutil.Logger, gFlags *flag.GlobalFlags) *cobra.Command {
	args := &Args{
		GlobalFlags: gFlags,
	}
	cmd := &cobra.Command{
		Use:   "revoke [<app name>...]",
		Short: "Revoke GitHub App User Access Tokens",
		Long: `Revoke GitHub App User Access Tokens via GitHub's credential revocation API and remove them from the backend.

Pass a raw token with --token-stdin so it does not appear in process arguments or shell history. One token is read from the first line of standard input. Positional raw tokens remain supported for compatibility but are unsafe because other processes and shell history may expose them.

Positional arguments that are not raw tokens are treated as app names whose stored tokens are revoked and removed from the backend.
When no argument is given, the token stored for GHTKN_APP (or the default app) is revoked.

With --all, the stored tokens of every app in the config are revoked. This is meant for incident response: when the environment running ghtkn is compromised, all stored tokens can be revoked at once. App name arguments are ignored when --all is set, but raw access tokens are still revoked as usual.`,
		Args: cobra.ArbitraryArgs,
		// Raw access tokens are arguments too, but only app names can be completed.
		ValidArgsFunction: completion.AppNames(&args.Config, ignoresAppNames),
		RunE: func(cmd *cobra.Command, positional []string) error {
			args.Args = positional
			args.TokenReader = cmd.InOrStdin()
			return action(cmd.Context(), logger, args)
		},
	}
	cmd.Flags().BoolVar(&args.All, "all", false, "Revoke the stored tokens of every app in the config")
	cmd.Flags().BoolVar(&args.TokenStdin, "token-stdin", false, "Read one raw access token from the first line of standard input (recommended for raw tokens)")
	return cmd
}

// ignoresAppNames reports whether the command line being completed makes revoke drop
// its app name arguments, which --all does (see action). Completing them there would
// only make them look like they still select what is revoked.
func ignoresAppNames(cmd *cobra.Command) bool {
	all, err := cmd.Flags().GetBool("all")
	// The flag is registered right above, so the error can only mean the command being
	// completed is not this one; offering no app name is the safe answer either way.
	return err == nil && all
}

// classify splits positional arguments into raw access tokens and app names by
// their prefix (see isToken). An argument that looks like a GitHub credential is a
// token; any other argument is an app name.
func classify(positional []string) (tokens, appNames []string) {
	for _, a := range positional {
		if isToken(a) {
			tokens = append(tokens, a)
		} else {
			appNames = append(appNames, a)
		}
	}
	return tokens, appNames
}

// isToken reports whether s looks like a GitHub credential based on its prefix.
// A positional argument that starts with one of these prefixes is treated as a raw
// access token rather than an app name.
func isToken(s string) bool {
	tokenPrefixes := []string{
		"ghp_",        // Personal access tokens (classic)
		"github_pat_", // Fine-grained personal access tokens
		"gho_",        // OAuth app access tokens
		"ghu_",        // User-to-server tokens from GitHub Apps
		"ghr_",        // Refresh tokens from GitHub Apps
	}
	for _, p := range tokenPrefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// readToken reads one newline-terminated or EOF-terminated raw token. The input is
// deliberately bounded and validated as a token so an app name cannot be
// accidentally redirected into the credential revocation API.
func readToken(r io.Reader) (string, error) {
	if r == nil {
		return "", errors.New("read token from standard input: input is unavailable")
	}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, initialTokenStdinBufferBytes), maxTokenStdinBytes+1)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("read token from standard input: %w", err)
		}
		return "", errors.New("read token from standard input: token is empty")
	}
	token := scanner.Text()
	if len(token) > maxTokenStdinBytes {
		return "", fmt.Errorf("read token from standard input: token exceeds %d bytes", maxTokenStdinBytes)
	}
	if token == "" {
		return "", errors.New("read token from standard input: token is empty")
	}
	if strings.IndexFunc(token, unicode.IsSpace) >= 0 {
		return "", errors.New("read token from standard input: expected exactly one token")
	}
	if !isToken(token) {
		return "", errors.New("read token from standard input: value does not have a recognized GitHub token prefix")
	}
	return token, nil
}

// action revokes the requested tokens.
// Positional arguments are classified into raw tokens (revoked directly) and app
// names (whose stored tokens are revoked via the SDK). When no argument is given,
// the SDK falls back to GHTKN_APP / the default app; when only raw tokens are
// given, the SDK is not called so a raw token never touches an unrelated app's
// stored token.
func action(ctx context.Context, logger *slogutil.Logger, args *Args) error {
	if err := logger.SetLevel(args.LogLevel); err != nil {
		return fmt.Errorf("set log level: %w", err)
	}

	tokens, appNames := classify(args.Args)
	if args.TokenStdin {
		token, err := readToken(args.TokenReader)
		if err != nil {
			return err
		}
		tokens = append(tokens, token)
	}
	if args.All {
		// --all revokes every app's stored token, so explicit app names are ignored.
		// Raw tokens are still revoked.
		appNames = nil
	}

	input, err := revoke.NewInput()
	if err != nil {
		return fmt.Errorf("create the controller input: %w", err)
	}
	p, err := config.ResolvePath(args.Config)
	if err != nil {
		return err //nolint:wrapcheck
	}
	return revoke.New(input).Run(ctx, logger.Logger, &revoke.InputRevoke{ //nolint:wrapcheck
		Tokens:         tokens,
		AppNames:       appNames,
		ConfigFilePath: p,
		All:            args.All,
	})
}
