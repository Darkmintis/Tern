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

func TestResolvePathEnvs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "secrets/play.json")
	t.Setenv("ANDROID_KEYSTORE", filepath.Join(dir, "abs.jks")) // already abs
	t.Setenv("BARE_TOKEN", "notapath")
	ResolvePathEnvs(dir, "GOOGLE_APPLICATION_CREDENTIALS", "ANDROID_KEYSTORE", "BARE_TOKEN")
	got := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	want := filepath.Join(dir, "secrets", "play.json")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if os.Getenv("ANDROID_KEYSTORE") != filepath.Join(dir, "abs.jks") {
		t.Fatalf("abs path rewritten: %s", os.Getenv("ANDROID_KEYSTORE"))
	}
	if os.Getenv("BARE_TOKEN") != "notapath" {
		t.Fatalf("non-path rewritten: %s", os.Getenv("BARE_TOKEN"))
	}
}

func TestLoadProject(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, ".env"), []byte("TERN_LOAD_PROJECT=yes\n"), 0o644)
	_ = os.Unsetenv("TERN_LOAD_PROJECT")
	if err := LoadProject(dir); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("TERN_LOAD_PROJECT"); got != "yes" {
		t.Fatalf("%q", got)
	}
}
