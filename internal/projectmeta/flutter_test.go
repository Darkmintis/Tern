package projectmeta_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/darkmintis/Tern/internal/projectmeta"
)

func TestFlutterVersion(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte("name: x\nversion: 1.2.3+4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	v, err := projectmeta.FlutterVersion(dir)
	if err != nil || v != "1.2.3+4" {
		t.Fatalf("%q %v", v, err)
	}
}
