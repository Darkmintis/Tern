package gradlecheck_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/darkmintis/Tern/internal/gradlecheck"
)

func TestFlutterAndroidSigningConfigured(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "android", "app")
	_ = os.MkdirAll(app, 0o755)
	_ = os.WriteFile(filepath.Join(app, "build.gradle"), []byte(`
key.properties
signingConfigs { release {} }
`), 0o644)
	if err := gradlecheck.FlutterAndroidSigningConfigured(dir); err != nil {
		t.Fatal(err)
	}
}

func TestMissingKeyPropertiesRef(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "android", "app")
	_ = os.MkdirAll(app, 0o755)
	_ = os.WriteFile(filepath.Join(app, "build.gradle"), []byte(`android {}`), 0o644)
	if err := gradlecheck.FlutterAndroidSigningConfigured(dir); err == nil {
		t.Fatal("expected error")
	}
}
