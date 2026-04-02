# AI Branch Metadata Design

## Summary

Duke Squad should create readable Git worktree branches that look like they were named by a small software team instead of by a random session handle.

When a user starts a Git-backed session on `New branch`, Duke Squad will:

1. Generate branch metadata with Codex using `gpt-5.4-mini`
2. Create a branch in the format `dev/<slug>`
3. Persist a short human-readable description for the session
4. Fall back to deterministic local naming if AI generation fails

This mirrors the spirit of T3 Code's AI-generated Git text workflow while fitting Duke Squad's current Go/TUI architecture.

## Goals

- Replace session-handle-based new branch names with readable `dev/<slug>` names
- Use Codex `gpt-5.4-mini` to generate branch metadata automatically
- Require no extra confirmation or manual edit step
- Keep existing branch selection behavior unchanged
- Preserve reliable startup with a deterministic fallback when Codex is unavailable or returns bad output

## Non-Goals

- Rebuild the full T3 Code git text-generation stack
- Add branch editing UI during session creation
- Generate commit messages or PR titles in this change
- Change behavior for non-Git projects

## Current State

- New Git sessions call `git.NewGitWorktreeWithRunner(...)`
- New branch names are currently derived from `config.BranchPrefix + sessionHandleName(...)`
- The branch picker only distinguishes an existing branch from `New branch`
- Session list secondary text currently shows the branch name only
- The prompted create flow (`N`) includes title, provider, prompt, and branch picker
- The fast create flow (`n`) includes title and provider only

## Desired Behavior

### Existing branches

If the user selects an existing branch, Duke Squad must keep using that branch exactly as it does now. No AI generation should run in this path.

### New branches

If the user selects `New branch` or creates a Git session from the fast flow without branch selection:

- Duke Squad generates branch metadata before creating the worktree
- The generated branch name must be `dev/<slug>`
- The generated description must be stored on the session and shown in the UI

### Input used for generation

Duke Squad should build one text prompt for branch generation from:

- Session title
- Initial prompt, if present
- Repository name

Rules:

- If the prompted flow was used, include both title and prompt
- If the fast flow was used, use title only
- The title should remain the user's title; AI generation only affects branch metadata

### Output contract

Codex must return structured JSON with:

- `slug`: short branch fragment only, without the `dev/` prefix
- `description`: one concise sentence fragment describing the work

Example:

```json
{
  "slug": "keep-preview-live",
  "description": "Keep the preview pane updating while scrolled"
}
```

### Normalization rules

Before branch creation:

- Force the final branch prefix to `dev/`
- Sanitize the generated slug with existing Git-safe rules
- Strip any accidental prefix returned by the model, such as `dev/` or `feature/`
- Collapse duplicate separators
- If the sanitized slug becomes empty, use a deterministic fallback slug from the session title

Description normalization:

- Trim whitespace
- Collapse newlines to spaces
- Keep it short enough for the TUI secondary line
- If empty after normalization, use a short title-derived fallback

## Failure Handling

AI generation must never block session creation permanently.

If Codex is missing, fails, times out, or returns invalid JSON:

- Log the failure
- Generate a deterministic fallback slug from the session title
- Create branch `dev/<fallback-slug>`
- Use the normalized session title as the fallback description

This fallback path should be local and require no network/model availability.

## Architecture

### Config

Default config should change the generated branch prefix from `<username>/` to `dev/`.

Requirements:

- New configs default to `"branch_prefix": "dev/"`
- Existing user-configured `branch_prefix` values must continue to load unchanged
- AI-generated branch creation should still force the `dev/` prefix for this feature path, regardless of the old session-handle naming logic

### Branch metadata generator

Add a small Go helper dedicated to branch metadata generation.

Responsibilities:

- Build the Codex prompt
- Write a temporary JSON schema file
- Execute `codex exec` non-interactively
- Request model `gpt-5.4-mini`
- Parse structured output
- Normalize slug and description
- Return fallback metadata on failure

Required files:

- `session/git/branch_metadata.go`
- `session/git/branch_metadata_test.go`

Return type:

```go
type BranchMetadata struct {
    BranchName  string
    Description string
}
```

### Worktree creation flow

The new metadata generator should be called only for new Git branches.

Required flow:

1. Session creation decides whether an existing branch was selected
2. If an existing branch was selected, keep current behavior
3. If a new branch is needed, generate branch metadata first
4. Pass the generated branch name into Git worktree creation
5. Store the generated description on the session

This means the branch-name decision can no longer live only inside `git.NewGitWorktreeWithRunner(...)` based on the session handle.

### Session model and persistence

Persist generated branch description on the session.

Required changes:

- Add a session field for branch description
- Save and restore it through storage
- Keep older saved sessions loading correctly when the field is absent

### UI

Show the generated description where it improves readability without adding a new workflow.

Minimum required places:

- Session list secondary line for Git sessions
- Instance-created help overlay

Rendering guidance:

- Branch name remains visible
- Description should appear as supporting text, not replace the branch
- If space is tight, truncate gracefully

## Prompt Design

The Codex prompt should be strict and minimal.

Required instructions:

- You generate git branch metadata for a software engineering task
- Return JSON with `slug` and `description`
- `slug` should be 2-6 plain words, lowercase, hyphenated, no prefix
- `description` should be a short readable summary for humans
- Use the user's requested work, not generic labels
- Avoid punctuation-heavy output, ticket prefixes, and filler words

Execution requirements:

- Use `codex exec`
- Use `--model gpt-5.4-mini`
- Use `--sandbox read-only`
- Use structured output via `--output-schema`

## Testing

### Unit tests

- Slug normalization strips prefixes like `dev/` and `feature/`
- Invalid model output falls back safely
- Empty or invalid slugs fall back to title-derived slugs
- Description normalization trims and shortens correctly
- Existing branch selection does not invoke AI generation
- Storage round-trip preserves branch description

### Integration-level behavior tests

- Prompted Git session on `New branch` creates `dev/<slug>`
- Fast Git session without prompt still creates `dev/<slug>` from title
- Non-Git sessions remain unchanged

## Rollout Notes

- This change is scoped to branch naming and branch descriptions only
- Commit/PR text generation is explicitly out of scope for this implementation

## Open Decisions Resolved

- No user edit step before branch creation
- Match the T3 Code behavior directionally, not by copying its codebase architecture
- Use `gpt-5.4-mini` specifically for branch metadata generation
- Keep descriptions automatic and read-only in this phase
