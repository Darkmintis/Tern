package projectmeta_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/darkmintis/Tern/internal/projectmeta"
)

func TestAndroidPackageID(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "android", "app")
	_ = os.MkdirAll(app, 0o755)
	_ = os.WriteFile(filepath.Join(app, "build.gradle"), []byte(`
android {
    namespace "com.example.ns"
    defaultConfig {
        applicationId "com.example.app"
    }
}
`), 0o644)
	id, err := projectmeta.AndroidPackageID(dir)
	if err != nil {
		t.Fatal(err)
	}
	if id != "com.example.app" {
		t.Fatalf("got %s", id)
	}
}

func TestAndroidPackageIDEnvOverride(t *testing.T) {
	t.Setenv("ANDROID_PACKAGE_NAME", "com.from.env")
	id, err := projectmeta.AndroidPackageID(t.TempDir())
	if err != nil || id != "com.from.env" {
		t.Fatalf("%s %v", id, err)
	}
}
