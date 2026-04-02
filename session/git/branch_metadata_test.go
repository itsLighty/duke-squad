package git

import (
	"bytes"
	"errors"
	"fmt"
	stdlog "log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"claude-squad/log"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeGeneratedBranchSlug(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "strips feature prefix and sanitizes",
			input: "feature/Launch New API!!",
			want:  "dev/launch-new-api",
		},
		{
			name:  "strips refs heads prefix",
			input: "refs/heads/bugfix/Resolve   Crash",
			want:  "dev/resolve-crash",
		},
		{
			name:  "keeps nested slug after removing accidental dev prefix",
			input: "dev/feature/Improve-Docs",
			want:  "dev/improve-docs",
		},
		{
			name:  "flattens slash and dot separators from model output",
			input: "platform/auth.v2",
			want:  "dev/platform-auth-v2",
		},
		{
			name:  "converts underscores to hyphens",
			input: "feature/user_profile_sync",
			want:  "dev/user-profile-sync",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, normalizeGeneratedBranchSlug(tt.input))
		})
	}
}

func TestNormalizeBranchDescription(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "collapses whitespace",
			input: "  Ship   the\nfeature\tquickly  ",
			want:  "Ship the feature quickly",
		},
		{
			name:  "truncates long descriptions",
			input: strings.Repeat("a", maxBranchDescriptionLength+12),
			want:  strings.Repeat("a", maxBranchDescriptionLength),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, normalizeBranchDescription(tt.input))
		})
	}
}

func TestGenerateBranchMetadata(t *testing.T) {
	t.Run("uses codex output when valid", func(t *testing.T) {
		original := runBranchMetadataGenerator
		t.Cleanup(func() {
			runBranchMetadataGenerator = original
		})

		runBranchMetadataGenerator = func(repoPath, repoName, title, prompt string) (string, error) {
			require.Equal(t, "/tmp/repo", repoPath)
			require.Equal(t, "project-x", repoName)
			require.Equal(t, "Different title", title)
			require.Equal(t, "Explain the change", prompt)
			return `{"slug":"feature/Launch   Feature!!","description":"  Ship\n  the   feature  quickly "}`, nil
		}

		got := GenerateBranchMetadata("/tmp/repo", "project-x", "Different title", "Explain the change")

		assert.Equal(t, BranchMetadata{
			BranchName:  "dev/launch-feature",
			Description: "Ship the feature quickly",
		}, got)
	})

	t.Run("keeps valid slug when description is empty", func(t *testing.T) {
		original := runBranchMetadataGenerator
		t.Cleanup(func() {
			runBranchMetadataGenerator = original
		})

		runBranchMetadataGenerator = func(repoPath, repoName, title, prompt string) (string, error) {
			return `{"slug":"feature/platform/auth","description":"   "}`, nil
		}

		got := GenerateBranchMetadata("/tmp/repo", "project-x", "Launch feature", "Explain the change")

		assert.Equal(t, BranchMetadata{
			BranchName:  "dev/platform-auth",
			Description: "Launch feature",
		}, got)
	})

	t.Run("keeps valid description when slug is empty", func(t *testing.T) {
		original := runBranchMetadataGenerator
		t.Cleanup(func() {
			runBranchMetadataGenerator = original
		})

		runBranchMetadataGenerator = func(repoPath, repoName, title, prompt string) (string, error) {
			return `{"slug":"   ","description":"  Ship   the\nfeature\tquickly  "}`, nil
		}

		got := GenerateBranchMetadata("/tmp/repo", "project-x", "Launch feature", "Explain the change")

		assert.Equal(t, BranchMetadata{
			BranchName:  "dev/launch-feature",
			Description: "Ship the feature quickly",
		}, got)
	})

	t.Run("falls back when codex fails", func(t *testing.T) {
		original := runBranchMetadataGenerator
		t.Cleanup(func() {
			runBranchMetadataGenerator = original
		})

		runBranchMetadataGenerator = func(repoPath, repoName, title, prompt string) (string, error) {
			return "", errors.New("codex unavailable")
		}

		got := GenerateBranchMetadata("/tmp/repo", "project-x", "Launch feature", "Explain the change")

		assert.Equal(t, BranchMetadata{
			BranchName:  "dev/launch-feature",
			Description: "Launch feature",
		}, got)
	})
}

func TestGenerateBranchMetadataFallbackLogging(t *testing.T) {
	type testCase struct {
		name            string
		raw             string
		wantBranchName  string
		wantDescription string
		wantLog         string
	}

	cases := []testCase{
		{
			name:            "malformed json falls back and logs",
			raw:             "{",
			wantBranchName:  "dev/launch-feature",
			wantDescription: "Launch feature",
			wantLog:         "failed to decode codex branch metadata",
		},
		{
			name:            "empty slug keeps description and logs",
			raw:             `{"slug":"   ","description":"Generated summary"}`,
			wantBranchName:  "dev/launch-feature",
			wantDescription: "Generated summary",
			wantLog:         "codex branch metadata missing slug",
		},
		{
			name:            "empty description keeps slug and logs",
			raw:             `{"slug":"feature/platform/auth","description":"   "}`,
			wantBranchName:  "dev/platform-auth",
			wantDescription: "Launch feature",
			wantLog:         "codex branch metadata missing description",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			originalRunner := runBranchMetadataGenerator
			t.Cleanup(func() {
				runBranchMetadataGenerator = originalRunner
			})

			var buf bytes.Buffer
			originalLog := log.ErrorLog
			t.Cleanup(func() {
				log.ErrorLog = originalLog
			})
			log.ErrorLog = stdlog.New(&buf, "", 0)

			runBranchMetadataGenerator = func(repoPath, repoName, title, prompt string) (string, error) {
				return tt.raw, nil
			}

			got := GenerateBranchMetadata("/tmp/repo", "project-x", "Launch feature", "Explain the change")

			assert.Equal(t, BranchMetadata{
				BranchName:  tt.wantBranchName,
				Description: tt.wantDescription,
			}, got)
			assert.Contains(t, buf.String(), tt.wantLog)
		})
	}
}

func TestRunCodexBranchMetadataGenerator(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is POSIX-specific")
	}

	tempDir := t.TempDir()
	codexPath := filepath.Join(tempDir, "codex")

	makeScript := func(promptFile, expectedRepoDir string, expectCD bool) string {
		cdCheck := `[ -z "$repo_dir" ]`
		if expectCD {
			cdCheck = fmt.Sprintf(`[ "$repo_dir" = %q ]`, expectedRepoDir)
		}

		return fmt.Sprintf(`#!/bin/sh
set -eu

expected_model=gpt-5.4-mini
expected_sandbox=read-only

subcommand=""
model=""
sandbox=""
repo_dir=""
schema_path=""
output_path=""
skip_git_repo_check=""
ephemeral=""

while [ "$#" -gt 0 ]; do
	case "$1" in
		exec) subcommand="$1" ;;
		--model) model="$2"; shift ;;
		--sandbox) sandbox="$2"; shift ;;
		--cd) repo_dir="$2"; shift ;;
		--output-schema) schema_path="$2"; shift ;;
		--output-last-message) output_path="$2"; shift ;;
		--skip-git-repo-check) skip_git_repo_check=1 ;;
		--ephemeral) ephemeral=1 ;;
		-) ;;
	esac
	shift
done

[ "$subcommand" = "exec" ]
[ "$model" = "$expected_model" ]
[ "$sandbox" = "$expected_sandbox" ]
%s
[ -n "$skip_git_repo_check" ]
[ -n "$ephemeral" ]
[ -f "$schema_path" ]
[ -n "$output_path" ]

grep -q '"slug"' "$schema_path"
! grep -q 'branch_name' "$schema_path"
grep -q '"description"' "$schema_path"

cat > %q
printf '{"slug":"feature/fake-branch","description":"  generated description  "}' > "$output_path"
`, cdCheck, promptFile)
	}

	runWithRepoPath := func(t *testing.T, repoPath string, expectCD bool) {
		t.Helper()
		promptFile := filepath.Join(tempDir, strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())+".prompt")

		require.NoError(t, os.WriteFile(codexPath, []byte(makeScript(promptFile, repoPath, expectCD)), 0o755))

		originalResolver := getCodexProgramCommand
		t.Cleanup(func() {
			getCodexProgramCommand = originalResolver
		})
		getCodexProgramCommand = func(name string) (string, error) {
			require.Equal(t, "codex", name)
			return codexPath, nil
		}

		raw, err := runCodexBranchMetadataGenerator(repoPath, "project-x", "Different title", "Explain the change")
		require.NoError(t, err)
		assert.JSONEq(t, `{"slug":"feature/fake-branch","description":"  generated description  "}`, raw)

		promptBytes, err := os.ReadFile(promptFile)
		require.NoError(t, err)
		prompt := string(promptBytes)
		assert.NotContains(t, prompt, repoPath)
		assert.Contains(t, prompt, "project-x")
		assert.Contains(t, prompt, "Different title")
		assert.Contains(t, prompt, "Explain the change")
	}

	t.Run("omits cd for non-local repo paths", func(t *testing.T) {
		runWithRepoPath(t, filepath.Join(tempDir, "remote", "repo.git"), false)
	})

	t.Run("passes cd for local repo paths", func(t *testing.T) {
		repoPath := filepath.Join(tempDir, "local-repo")
		require.NoError(t, os.MkdirAll(repoPath, 0o755))
		runWithRepoPath(t, repoPath, true)
	})
}
