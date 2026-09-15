package ternerrors_test

import (
	"errors"
	"fmt"
	"testing"

	ternerrors "github.com/darkmintis/Tern/internal/errors"
)

func TestWrapAndClass(t *testing.T) {
	base := fmt.Errorf("boom")
	err := ternerrors.Wrap(ternerrors.ClassBuild, "flutter", base)
	if ternerrors.ExitCode(err) != 4 {
		t.Fatalf("exit %d", ternerrors.ExitCode(err))
	}
	class, ok := ternerrors.AsClass(err)
	if !ok || class != ternerrors.ClassBuild {
		t.Fatalf("%v %v", class, ok)
	}
	if !errors.Is(err, base) {
		t.Fatal("unwrap broken")
	}
}

func TestHint(t *testing.T) {
	err := ternerrors.NewHint(ternerrors.ClassUpload, "missing creds", "set GOOGLE_APPLICATION_CREDENTIALS")
	if ternerrors.HintOf(err) != "set GOOGLE_APPLICATION_CREDENTIALS" {
		t.Fatal(ternerrors.HintOf(err))
	}
	if ternerrors.ExitCode(err) != 6 {
		t.Fatal(ternerrors.ExitCode(err))
	}
}

func TestExitCodeUnknown(t *testing.T) {
	if ternerrors.ExitCode(fmt.Errorf("x")) != 1 {
		t.Fatal()
	}
}

func TestNew(t *testing.T) {
	err := ternerrors.New(ternerrors.ClassBuild, "compile failed")
	if got := err.Error(); got != "BuildError: compile failed" {
		t.Fatalf("got %q", got)
	}
	class, ok := ternerrors.AsClass(err)
	if !ok || class != ternerrors.ClassBuild {
		t.Fatal(class, ok)
	}
}

func TestTruncateStderr(t *testing.T) {
	short := ternerrors.TruncateStderr("hello")
	if short != "hello" {
		t.Fatal(short)
	}
	empty := ternerrors.TruncateStderr("")
	if empty != "" {
		t.Fatal(empty)
	}
	big := make([]byte, 20000)
	for i := range big {
		big[i] = 'x'
	}
	trunc := ternerrors.TruncateStderr(string(big))
	if len(trunc) > 16384+50 {
		t.Fatalf("expected truncation, got len %d", len(trunc))
	}
}

func TestStderrOf(t *testing.T) {
	if ternerrors.StderrOf(nil) != "" {
		t.Fatal("nil should return empty")
	}
	if ternerrors.StderrOf(fmt.Errorf("plain")) != "" {
		t.Fatal("plain error should return empty")
	}
	err := ternerrors.WrapStderr(ternerrors.ClassExec, "build", "some stderr", fmt.Errorf("fail"))
	if got := ternerrors.StderrOf(err); got != "some stderr" {
		t.Fatal(got)
	}
}

func TestMessageOf(t *testing.T) {
	if ternerrors.MessageOf(nil) != "" {
		t.Fatal("nil should return empty")
	}
	err := ternerrors.Wrap(ternerrors.ClassBuild, "compile failed", fmt.Errorf("boom"))
	if got := ternerrors.MessageOf(err); got != "compile failed" {
		t.Fatal(got)
	}
	plain := fmt.Errorf("plain error")
	if got := ternerrors.MessageOf(plain); got != "plain error" {
		t.Fatal(got)
	}
}

func TestDetailOf(t *testing.T) {
	if ternerrors.DetailOf(nil) != "" {
		t.Fatal("nil should return empty")
	}
	err := ternerrors.NewHint(ternerrors.ClassUpload, "missing creds", "set GOOGLE_APPLICATION_CREDENTIALS")
	detail := ternerrors.DetailOf(err)
	if detail == "" {
		t.Fatal("expected non-empty detail")
	}
}
