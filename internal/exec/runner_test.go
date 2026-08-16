package execx_test

import (
	"bytes"
	"context"
	"os/exec"
	"testing"

	ternerrors "github.com/darkmintis/Tern/internal/errors"
	execx "github.com/darkmintis/Tern/internal/exec"
)

// goBin returns a go binary for the test (cross-platform).
func goBin(t *testing.T) string {
	t.Helper()
	p, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRealRunnerSuccess(t *testing.T) {
	r := &execx.RealRunner{}
	out, err := r.Run(context.Background(), t.TempDir(), goBin(t), "version")
	if err != nil {
		t.Fatal(err)
	}
	if out == "" {
		t.Fatal("expected stdout")
	}
}

func TestRealRunnerFailureClassifiesStderr(t *testing.T) {
	r := &execx.RealRunner{}
	_, err := r.Run(context.Background(), t.TempDir(), goBin(t), "definitely-not-a-go-command-9494")
	if err == nil {
		t.Fatal("expected error")
	}
	if class, ok := ternerrors.AsClass(err); !ok || class != ternerrors.ClassExec {
		t.Fatalf("class=%q", class)
	}
	if ternerrors.StderrOf(err) == "" {
		t.Fatal("expected captured stderr")
	}
}

func TestRealRunnerStreamsToWriters(t *testing.T) {
	var out, errw bytes.Buffer
	r := &execx.RealRunner{Stdout: &out, Stderr: &errw}
	if _, err := r.Run(context.Background(), t.TempDir(), goBin(t), "version"); err != nil {
		t.Fatal(err)
	}
	if out.Len() == 0 {
		t.Fatal("expected stdout streamed")
	}
}

func TestRealRunnerMissingBinary(t *testing.T) {
	r := &execx.RealRunner{}
	_, err := r.Run(context.Background(), t.TempDir(), "no-such-binary-tern-xyz-123")
	if err == nil {
		t.Fatal("expected error")
	}
	if class, ok := ternerrors.AsClass(err); !ok || class != ternerrors.ClassExec {
		t.Fatalf("class=%q", class)
	}
}

func TestVerboseEnvControl(t *testing.T) {
	execx.SetVerbose(false)
	defer execx.SetVerbose(false)

	t.Setenv("TERN_VERBOSE", "1")
	if !execx.Verbose() {
		t.Fatal("TERN_VERBOSE=1 should be verbose")
	}
	t.Setenv("TERN_VERBOSE", "true")
	if !execx.Verbose() {
		t.Fatal("TERN_VERBOSE=true should be verbose")
	}
	t.Setenv("TERN_VERBOSE", "0")
	if execx.Verbose() {
		t.Fatal("TERN_VERBOSE=0 should not be verbose")
	}
	t.Setenv("TERN_VERBOSE", "")
	if execx.Verbose() {
		t.Fatal("unset TERN_VERBOSE should not be verbose")
	}
}

func TestSetVerboseOverridesEnv(t *testing.T) {
	defer execx.SetVerbose(false)
	t.Setenv("TERN_VERBOSE", "")
	execx.SetVerbose(true)
	if !execx.Verbose() {
		t.Fatal("SetVerbose(true) should force verbose")
	}
	execx.SetVerbose(false)
	if execx.Verbose() {
		t.Fatal("SetVerbose(false) should force non-verbose")
	}
}

func TestLookPathMissing(t *testing.T) {
	if _, err := execx.LookPath("no-such-binary-tern-xyz-123"); err == nil {
		t.Fatal("expected LookPath error")
	}
}
