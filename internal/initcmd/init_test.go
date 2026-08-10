package initcmd_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/darkmintis/Tern/internal/adapter"
	"github.com/darkmintis/Tern/internal/adapter/flutter"
	initcmd "github.com/darkmintis/Tern/internal/initcmd"
)

func TestInitCreatesTernfileAndWorkflow(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte("dependencies:\n  flutter:\n    sdk: flutter\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, ".metadata"), []byte("v:1\n"), 0o644)

	res, err := initcmd.Run(dir, adapter.NewRegistry(flutter.New(nil)), true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Adapter != "flutter" {
		t.Fatalf("%s", res.Adapter)
	}
	if _, err := os.Stat(filepath.Join(dir, "Ternfile")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".github", "workflows", "tern-release.yml")); err != nil {
		t.Fatal(err)
	}
	_, err = initcmd.Run(dir, adapter.NewRegistry(flutter.New(nil)), false)
	if err == nil {
		t.Fatal("expected already exists")
	}
}
