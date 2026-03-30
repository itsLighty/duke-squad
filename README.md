# Duke Squad [![CI](https://github.com/itsLighty/duke-squad/actions/workflows/build.yml/badge.svg)](https://github.com/itsLighty/duke-squad/actions/workflows/build.yml) [![GitHub Release](https://img.shields.io/github/v/release/itsLighty/duke-squad)](https://github.com/itsLighty/duke-squad/releases/latest)

[Duke Squad](https://github.com/itsLighty/duke-squad) is a terminal app that manages multiple AI coding agents in parallel across local folders and remote SSH projects.

![Duke Squad Screenshot](assets/screenshot.png)

### Highlights

- Manage local folders and SSH-backed projects in one TUI
- Run Claude Code, Codex, Gemini, Aider, and similar agents side-by-side
- Isolate Git work with worktrees and folder work with managed snapshots
- Review preview, diff, and terminal panes before pushing changes

<br />

https://github.com/user-attachments/assets/aef18253-e58f-4525-9032-f5a3d66c975a

<br />

### Installation

#### Shell Script

```bash
curl -fsSL https://raw.githubusercontent.com/itsLighty/duke-squad/main/install.sh | bash
```

This installs the canonical `duke-squad` binary and a `ds` shortcut into `~/.local/bin`.

If upstream `cs` is already installed, the installer leaves it alone and warns that the fork should be launched with `duke-squad` or `ds`.

#### Local Build

```bash
go build -o build/duke-squad .
mkdir -p ~/.local/bin
ln -sfn "$PWD/build/duke-squad" ~/.local/bin/duke-squad
ln -sfn "$PWD/build/duke-squad" ~/.local/bin/ds
```

### Prerequisites

- [tmux](https://github.com/tmux/tmux/wiki/Installing)
- [gh](https://cli.github.com/)

### Usage

```text
Usage:
  duke-squad [flags]
  duke-squad [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  debug       Print debug information like config paths
  help        Help about any command
  reset       Reset all stored instances
  version     Print the version number of duke-squad

Flags:
  -y, --autoyes          [experimental] If enabled, all instances will automatically accept prompts
  -h, --help             help for duke-squad
  -p, --program string   Preselected provider/program for new sessions in this run (e.g. 'codex')
```

Run the application with:

```bash
ds
```

`duke-squad` works as the full command if you prefer the canonical binary name.

### Adding Projects

Press `a` to open the add-project flow.

#### Local

- Leave the source picker on `Local`
- Enter the project folder path
- Duke Squad classifies the folder as Git-backed or plain-folder automatically

#### Remote SSH

- Switch the source picker to `Remote`
- Fill in `Project name` if you want a custom label
- Enter `SSH username`
- Enter `Host / IP`
- Enter `Password` only if key-based auth is not already configured
- Enter `Remote folder`

If the remote folder is a Git repository, Duke Squad creates remote worktrees. If it is a plain folder, Duke Squad creates a managed snapshot workspace on the remote host.

### Using Other Agents

- Codex: `ds -p "codex"`
- Aider: `ds -p "aider ..."`
- Gemini: `ds -p "gemini"`

Profiles in the config file let you expose multiple agent presets in the new-session picker and choose a default.

### Configuration

Duke Squad stores configuration in `~/.duke-squad/config.json`.

On first launch, if `~/.duke-squad` does not exist but `~/.claude-squad` does, Duke Squad copies:

- `config.json`
- `state.json`
- `instances.json`

This preserves existing local state without moving old worktree or managed-workspace directories.

#### Profiles

```json
{
  "default_program": "claude",
  "profiles": [
    { "name": "claude", "program": "claude" },
    { "name": "codex", "program": "codex" },
    { "name": "gemini", "program": "gemini" }
  ]
}
```

#### Auto-Yes

New configs default `auto_yes` to `true`. Set `"auto_yes": false` to opt out.

### How It Works

1. `tmux` creates isolated terminal sessions for each agent.
2. Git projects use worktrees so each session gets its own branch and workspace.
3. Folder projects use managed snapshots so agents can work without mutating the source folder directly.
4. SSH projects reuse the same model across remote runners.

### License

[AGPL-3.0](LICENSE.md)
