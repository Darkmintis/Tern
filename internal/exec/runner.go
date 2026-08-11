package execx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync/atomic"

	ternerrors "github.com/darkmintis/Tern/internal/errors"
)

// Runner executes external commands (mockable boundary).
type Runner interface {
	Run(ctx context.Context, dir string, name string, args ...string) (stdout string, err error)
}

var verboseFlag atomic.Bool

// SetVerbose controls whether command stderr is streamed live (and full logs on failure).
func SetVerbose(v bool) { verboseFlag.Store(v) }

// Verbose is true when --verbose was set or TERN_VERBOSE=1.
func Verbose() bool {
	if verboseFlag.Load() {
		return true
	}
	return os.Getenv("TERN_VERBOSE") == "1" || os.Getenv("TERN_VERBOSE") == "true"
}

// RealRunner shells out via os/exec.
type RealRunner struct {
	Stdout io.Writer
	Stderr io.Writer
}

func (r *RealRunner) Run(ctx context.Context, dir string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	if r.Stdout != nil {
		cmd.Stdout = io.MultiWriter(&outBuf, r.Stdout)
	}
	// Always capture stderr for diagnosis. Stream live only in verbose mode.
	cmd.Stderr = &errBuf
	if r.Stderr != nil && Verbose() {
		cmd.Stderr = io.MultiWriter(&errBuf, r.Stderr)
	}
	if err := cmd.Run(); err != nil {
		stderr := errBuf.String()
		return outBuf.String(), ternerrors.WrapStderr(ternerrors.ClassExec,
			fmt.Sprintf("running %s %v", name, args), stderr, err)
	}
	return outBuf.String(), nil
}

// LookPath checks if a binary exists on PATH.
func LookPath(name string) (string, error) {
	return exec.LookPath(name)
}
