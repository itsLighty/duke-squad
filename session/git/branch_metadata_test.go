package git

import (
	"errors"
	"testing"

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, normalizeGeneratedBranchSlug(tt.input))
		})
	}
}

func TestNormalizeBranchDescription(t *testing.T) {
	got := normalizeBranchDescription("  Ship   the\nfeature\tquickly  ")

	assert.Equal(t, "Ship the feature quickly", got)
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
			require.Equal(t, "Launch feature", title)
			require.Equal(t, "Explain the change", prompt)
			return `{"branch_name":"feature/Launch   Feature!!","description":"  Ship\n  the   feature  quickly "}`, nil
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
