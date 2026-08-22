package revoke

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestClassify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		positional   []string
		wantTokens   []string
		wantAppNames []string
	}{
		{
			name: "empty",
		},
		{
			name:       "token prefixes are tokens",
			positional: []string{"ghp_x", "github_pat_x", "gho_x", "ghu_x", "ghr_x"},
			wantTokens: []string{"ghp_x", "github_pat_x", "gho_x", "ghu_x", "ghr_x"},
		},
		{
			name:         "non-prefixed args are app names",
			positional:   []string{"my-app", "another"},
			wantAppNames: []string{"my-app", "another"},
		},
		{
			name:         "mixed and order preserved",
			positional:   []string{"ghu_a", "my-app", "ghp_b", "other"},
			wantTokens:   []string{"ghu_a", "ghp_b"},
			wantAppNames: []string{"my-app", "other"},
		},
		{
			name:       "a token-shaped app name is treated as a token",
			positional: []string{"ghu_looks-like-app"},
			wantTokens: []string{"ghu_looks-like-app"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tokens, appNames := classify(tt.positional)
			if diff := cmp.Diff(tt.wantTokens, tokens); diff != "" {
				t.Errorf("tokens mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantAppNames, appNames); diff != "" {
				t.Errorf("app names mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestReadToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "token ending at EOF",
			input: "ghu_test-token",
			want:  "ghu_test-token",
		},
		{
			name:  "token ending with newline",
			input: "github_pat_test-token\n",
			want:  "github_pat_test-token",
		},
		{
			name:  "token ending with CRLF",
			input: "ghp_test-token\r\n",
			want:  "ghp_test-token",
		},
		{
			name:    "empty",
			wantErr: true,
		},
		{
			name:    "app name rejected",
			input:   "my-app\n",
			wantErr: true,
		},
		{
			name:  "only first line read",
			input: "ghu_first\nghu_second\n",
			want:  "ghu_first",
		},
		{
			name:    "oversized input rejected",
			input:   "ghu_" + strings.Repeat("x", maxTokenStdinBytes),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := readToken(strings.NewReader(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("readToken() = %q, want an error", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("readToken() = %q, want %q", got, tt.want)
			}
		})
	}
}
