package dotenv

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFile_DoesNotOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	_ = os.WriteFile(path, []byte("TERN_DOTENV_A=fromfile\nTERN_DOTENV_B=onlyfile\n"), 0o644)
	t.Setenv("TERN_DOTENV_A", "fromshell")
	_ = os.Unsetenv("TERN_DOTENV_B")

	if err := LoadFile(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("TERN_DOTENV_A"); got != "fromshell" {
		t.Fatalf("override: %q", got)
	}
	if got := os.Getenv("TERN_DOTENV_B"); got != "onlyfile" {
		t.Fatalf("load: %q", got)
	}
}

func TestLoadFile_MissingOK(t *testing.T) {
	if err := LoadFile(filepath.Join(t.TempDir(), "nope.env")); err != nil {
		t.Fatal(err)
	}
}
