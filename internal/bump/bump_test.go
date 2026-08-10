package bump_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/darkmintis/Tern/internal/bump"
	"github.com/darkmintis/Tern/internal/config"
)

func TestBumpPubspec(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pubspec.yaml")
	_ = os.WriteFile(path, []byte("name: app\nversion: 1.2.3+4\n"), 0o644)
	res, err := bump.BumpVersion(dir, config.BumpPatch, false)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "version: 1.2.4+") {
		t.Fatalf("got %s message=%s", data, res.Message)
	}
}

func TestBumpDryRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pubspec.yaml")
	orig := "name: app\nversion: 1.0.0+1\n"
	_ = os.WriteFile(path, []byte(orig), 0o644)
	_, err := bump.BumpVersion(dir, config.BumpMajor, true)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != orig {
		t.Fatal("dry-run mutated file")
	}
}
