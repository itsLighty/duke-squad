package transport

import (
	"fmt"
	"os"
	"strings"
)

type Kind string

const (
	KindLocal Kind = "local"
	KindSSH   Kind = "ssh"
)

type CommandSpec struct {
	Program string
	Args    []string
	Dir     string
	Env     []string
}

type Runner interface {
	Kind() Kind
	Target() string
	Run(spec CommandSpec) error
	Output(spec CommandSpec) ([]byte, error)
	CombinedOutput(spec CommandSpec) ([]byte, error)
	StartPTY(spec CommandSpec) (*os.File, error)
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func shellCommand(spec CommandSpec) string {
	parts := make([]string, 0, 1+len(spec.Args))
	if len(spec.Env) > 0 {
		envParts := make([]string, 0, len(spec.Env))
		for _, entry := range spec.Env {
			name, value, found := strings.Cut(entry, "=")
			if !found {
				continue
			}
			envParts = append(envParts, fmt.Sprintf("export %s=%s", name, shellQuote(value)))
		}
		if len(envParts) > 0 {
			parts = append(parts, strings.Join(envParts, " && "))
		}
	}

	var command strings.Builder
	command.WriteString("exec ")
	command.WriteString(shellQuote(spec.Program))
	for _, arg := range spec.Args {
		command.WriteByte(' ')
		command.WriteString(shellQuote(arg))
	}

	if spec.Dir != "" {
		parts = append(parts, shellDirCommand(spec.Dir))
	}
	parts = append(parts, command.String())
	return strings.Join(parts, " && ")
}

func shellDirCommand(dir string) string {
	switch {
	case dir == "~":
		return "cd \"$HOME\""
	case strings.HasPrefix(dir, "~/"):
		return "cd \"$HOME\"/" + shellQuote(strings.TrimPrefix(dir, "~/"))
	default:
		return "cd " + shellQuote(dir)
	}
}
