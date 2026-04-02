package git

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"claude-squad/config"
)

const codexBranchModel = "gpt-5.4-mini"

// BranchMetadata captures the AI-generated branch name and description for a task.
type BranchMetadata struct {
	BranchName  string `json:"branch_name"`
	Description string `json:"description"`
}

var runBranchMetadataGenerator = runCodexBranchMetadataGenerator

// GenerateBranchMetadata asks Codex for branch metadata and falls back locally when Codex is unavailable.
func GenerateBranchMetadata(repoPath, repoName, title, prompt string) BranchMetadata {
	fallback := localBranchMetadata(repoName, title)

	raw, err := runBranchMetadataGenerator(repoPath, repoName, title, prompt)
	if err != nil {
		return fallback
	}

	var generated BranchMetadata
	if err := json.Unmarshal([]byte(raw), &generated); err != nil {
		return fallback
	}

	branchName := normalizeGeneratedBranchSlug(generated.BranchName)
	if branchName == "" {
		branchName = fallback.BranchName
	}

	description := normalizeBranchDescription(generated.Description)
	if description == "" {
		description = fallback.Description
	}

	return BranchMetadata{
		BranchName:  branchName,
		Description: description,
	}
}

func runCodexBranchMetadataGenerator(repoPath, repoName, title, prompt string) (string, error) {
	codexPath, err := config.GetProgramCommand("codex")
	if err != nil {
		return "", err
	}

	schemaPath, cleanupSchema, err := writeCodexOutputSchema()
	if err != nil {
		return "", err
	}
	defer cleanupSchema()

	outputPath, cleanupOutput, err := createCodexOutputFile()
	if err != nil {
		return "", err
	}
	defer cleanupOutput()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, codexPath,
		"exec",
		"--model", codexBranchModel,
		"--cd", repoPath,
		"--skip-git-repo-check",
		"--output-schema", schemaPath,
		"--output-last-message", outputPath,
		"--ephemeral",
		"-",
	)
	cmd.Stdin = strings.NewReader(codexMetadataPrompt(repoPath, repoName, title, prompt))

	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("codex branch metadata generation failed: %w: %s", err, strings.TrimSpace(string(output)))
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		return "", fmt.Errorf("codex branch metadata output missing: %w", err)
	}

	raw := strings.TrimSpace(string(data))
	if raw == "" {
		return "", fmt.Errorf("codex branch metadata output was empty")
	}

	return raw, nil
}

func writeCodexOutputSchema() (string, func(), error) {
	file, err := os.CreateTemp("", "branch-metadata-schema-*.json")
	if err != nil {
		return "", func() {}, err
	}

	schema := []byte(`{
  "type": "object",
  "properties": {
    "branch_name": { "type": "string" },
    "description": { "type": "string" }
  },
  "required": ["branch_name", "description"],
  "additionalProperties": false
}`)
	if _, err := file.Write(schema); err != nil {
		file.Close()
		os.Remove(file.Name())
		return "", func() {}, err
	}
	if err := file.Close(); err != nil {
		os.Remove(file.Name())
		return "", func() {}, err
	}

	return file.Name(), func() {
		_ = os.Remove(file.Name())
	}, nil
}

func createCodexOutputFile() (string, func(), error) {
	file, err := os.CreateTemp("", "branch-metadata-output-*.json")
	if err != nil {
		return "", func() {}, err
	}
	if err := file.Close(); err != nil {
		os.Remove(file.Name())
		return "", func() {}, err
	}

	return file.Name(), func() {
		_ = os.Remove(file.Name())
	}, nil
}

func codexMetadataPrompt(repoPath, repoName, title, prompt string) string {
	return strings.TrimSpace(fmt.Sprintf(`Generate branch metadata for a coding task.

Repository path: %s
Repository name: %s
Title: %s
Prompt: %s

Return only JSON that matches the schema.
Choose a concise branch slug without any environment prefix.
Choose a short human-readable description.
`, repoPath, repoName, title, prompt))
}

func localBranchMetadata(repoName, title string) BranchMetadata {
	branchSource := strings.TrimSpace(title)
	if branchSource == "" {
		branchSource = strings.TrimSpace(repoName)
	}

	branchName := normalizeGeneratedBranchSlug(branchSource)
	if branchName == "" {
		branchName = "dev/branch"
	}

	return BranchMetadata{
		BranchName:  branchName,
		Description: normalizeBranchDescription(title),
	}
}

func normalizeGeneratedBranchSlug(raw string) string {
	slug := strings.ToLower(strings.TrimSpace(raw))
	if slug == "" {
		return ""
	}

	for {
		stripped := false
		for _, prefix := range []string{"refs/heads/", "dev/", "feature/", "feat/", "fix/", "bugfix/", "chore/"} {
			if strings.HasPrefix(slug, prefix) {
				slug = strings.TrimPrefix(slug, prefix)
				stripped = true
				break
			}
		}
		if !stripped {
			break
		}
	}

	slug = sanitizeBranchName(slug)
	if slug == "" {
		return ""
	}

	return "dev/" + slug
}

func normalizeBranchDescription(raw string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
}
