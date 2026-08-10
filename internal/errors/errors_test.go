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
