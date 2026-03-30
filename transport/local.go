package transport

import (
	cscmd "claude-squad/cmd"
	"os"
	"os/exec"

	"github.com/creack/pty"
)

type PTYStarter interface {
	Start(cmd *exec.Cmd) (*os.File, error)
	Close()
}

type systemPTY struct{}

func (systemPTY) Start(cmd *exec.Cmd) (*os.File, error) {
	return pty.Start(cmd)
}

func (systemPTY) Close() {}

type LocalRunner struct {
	cmdExec cscmd.Executor
	pty     PTYStarter
}

func NewLocalRunner() *LocalRunner {
	return NewLocalRunnerWithDeps(cscmd.MakeExecutor(), systemPTY{})
}

func NewLocalRunnerWithDeps(cmdExec cscmd.Executor, pty PTYStarter) *LocalRunner {
	return &LocalRunner{cmdExec: cmdExec, pty: pty}
}

func (r *LocalRunner) Kind() Kind {
	return KindLocal
}

func (r *LocalRunner) Target() string {
	return ""
}

func (r *LocalRunner) Run(spec CommandSpec) error {
	return r.cmdExec.Run(r.command(spec))
}

func (r *LocalRunner) Output(spec CommandSpec) ([]byte, error) {
	return r.cmdExec.Output(r.command(spec))
}

func (r *LocalRunner) CombinedOutput(spec CommandSpec) ([]byte, error) {
	return r.command(spec).CombinedOutput()
}

func (r *LocalRunner) StartPTY(spec CommandSpec) (*os.File, error) {
	return r.pty.Start(r.command(spec))
}

func (r *LocalRunner) command(spec CommandSpec) *exec.Cmd {
	cmd := exec.Command(spec.Program, spec.Args...)
	cmd.Dir = spec.Dir
	if len(spec.Env) > 0 {
		cmd.Env = append(os.Environ(), spec.Env...)
	}
	return cmd
}
