package execx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"

	ternerrors "github.com/darkmintis/Tern/internal/errors"
)

// Runner executes external commands (mockable boundary).
type Runner interface {
	Run(ctx context.Context, dir string, name string, args ...string) (stdout string, err error)
}

// RealRunner shells out via os/exec.
type RealRunner struct {
	Stdout io.Writer
	Stderr io.Writer
}

func (r *RealRunner) Run(ctx context.Context, dir string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	if r.Stdout != nil {
		cmd.Stdout = io.MultiWriter(&buf, r.Stdout)
	}
	cmd.Stderr = r.Stderr
	if cmd.Stderr == nil {
		cmd.Stderr = &buf
	}
	if err := cmd.Run(); err != nil {
		return buf.String(), ternerrors.Wrap(ternerrors.ClassExec,
			fmt.Sprintf("running %s %v", name, args), err)
	}
	return buf.String(), nil
}

// LookPath checks if a binary exists on PATH.
func LookPath(name string) (string, error) {
	return exec.LookPath(name)
}
