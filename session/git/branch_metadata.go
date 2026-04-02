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
	"claude-squad/log"
)

const codexBranchModel = "gpt-5.4-mini"
const maxBranchDescriptionLength = 80

type codexBranchMetadata struct {
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

// BranchMetadata captures the AI-generated branch name and description for a task.
type BranchMetadata struct {
	BranchName  string `json:"branch_name"`
	Description string `json:"description"`
}

var runBranchMetadataGenerator = runCodexBranchMetadataGenerator
var getCodexProgramCommand = config.GetProgramCommand

// GenerateBranchMetadata asks Codex for branch metadata and falls back locally when Codex is unavailable.
func GenerateBranchMetadata(repoPath, repoName, title, prompt string) BranchMetadata {
	fallback := localBranchMetadata(repoName, title)

	raw, err := runBranchMetadataGenerator(repoPath, repoName, title, prompt)
	if err != nil {
		logBranchMetadataFallback("failed to generate codex branch metadata", err)
		return fallback
	}

	var generated codexBranchMetadata
	if err := json.Unmarshal([]byte(raw), &generated); err != nil {
		logBranchMetadataFallback("failed to decode codex branch metadata", err)
		return fallback
	}

	branchName := normalizeGeneratedBranchSlug(generated.Slug)
	if branchName == "" {
		logBranchMetadataFallback("codex branch metadata missing slug", nil)
		branchName = fallback.BranchName
	}

	description := normalizeBranchDescription(generated.Description)
	if description == "" {
		logBranchMetadataFallback("codex branch metadata missing description", nil)
		description = fallback.Description
	}

	return BranchMetadata{
		BranchName:  branchName,
		Description: description,
	}
}

func runCodexBranchMetadataGenerator(repoPath, repoName, title, prompt string) (string, error) {
	codexPath, err := getCodexProgramCommand("codex")
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

	args := []string{
		"exec",
		"--model", codexBranchModel,
		"--sandbox", "read-only",
		"--skip-git-repo-check",
		"--output-schema", schemaPath,
		"--output-last-message", outputPath,
		"--ephemeral",
		"-",
	}

	cmd := exec.CommandContext(ctx, codexPath, args...)
	neutralDir, err := os.MkdirTemp("", "codex-branch-metadata-*")
	if err != nil {
		return "", fmt.Errorf("codex branch metadata neutral directory failed: %w", err)
	}
	defer os.RemoveAll(neutralDir)
	cmd.Dir = neutralDir
	cmd.Stdin = strings.NewReader(codexMetadataPrompt(repoName, title, prompt))

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
    "slug": { "type": "string" },
    "description": { "type": "string" }
  },
  "required": ["slug", "description"],
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

func codexMetadataPrompt(repoName, title, prompt string) string {
	return strings.TrimSpace(fmt.Sprintf(`Generate branch metadata for a coding task.

Repository name: %s
Title: %s
Prompt: %s

Return only JSON that matches the schema.
Choose a concise branch slug without any environment prefix.
Choose a short human-readable description.
`, repoName, title, prompt))
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

	slug = strings.NewReplacer("/", "-", ".", "-", "_", "-").Replace(slug)
	slug = sanitizeBranchName(slug)
	if slug == "" {
		return ""
	}

	return "dev/" + slug
}

func normalizeBranchDescription(raw string) string {
	description := strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
	runes := []rune(description)
	if len(runes) > maxBranchDescriptionLength {
		description = string(runes[:maxBranchDescriptionLength])
	}
	return description
}

func logBranchMetadataFallback(message string, err error) {
	if log.WarningLog == nil {
		return
	}
	if err != nil {
		log.WarningLog.Printf("%s: %v", message, err)
		return
	}
	log.WarningLog.Printf("%s", message)
}
