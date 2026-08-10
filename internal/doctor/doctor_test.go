package doctor_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/darkmintis/Tern/internal/adapter"
	"github.com/darkmintis/Tern/internal/adapter/flutter"
	"github.com/darkmintis/Tern/internal/config"
	"github.com/darkmintis/Tern/internal/doctor"
	"github.com/darkmintis/Tern/internal/output"
)

func TestDoctorFlutterAndroid(t *testing.T) {
	if _, err := exec.LookPath("flutter"); err != nil {
		t.Skip("flutter not on PATH")
	}
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte("dependencies:\n  flutter:\n    sdk: flutter\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, ".metadata"), []byte("version: 1\n"), 0o644)
	app := filepath.Join(dir, "android", "app")
	_ = os.MkdirAll(app, 0o755)
	_ = os.WriteFile(filepath.Join(app, "build.gradle"), []byte(`
def keystoreProperties = new Properties()
def keystorePropertiesFile = rootProject.file('key.properties')
android {
  namespace "com.example.app"
  defaultConfig { applicationId "com.example.app" }
  signingConfigs {
    release {
      keyAlias keystoreProperties['keyAlias']
    }
  }
}
`), 0o644)
	ks := filepath.Join(dir, "ks.jks")
	_ = os.WriteFile(ks, []byte("k"), 0o600)
	creds := filepath.Join(dir, "play.json")
	_ = os.WriteFile(creds, []byte(`{}`), 0o600)

	tern := `
lane release:
  sign android with keystore env:ANDROID_KEYSTORE
  build android release
  upload android to play_store track:internal
`
	_ = os.WriteFile(filepath.Join(dir, "Ternfile"), []byte(tern), 0o644)
	t.Setenv("ANDROID_KEYSTORE", ks)
	t.Setenv("ANDROID_KEYSTORE_PASSWORD", "store-pass-strong")
	t.Setenv("ANDROID_KEY_ALIAS", "upload")
	t.Setenv("ANDROID_KEY_PASSWORD", "key-pass-strong")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", creds)

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	checks, err := doctor.Run(doctor.Options{
		ProjectRoot: dir,
		Config:      cfg,
		Registry:    adapter.NewRegistry(flutter.New(nil)),
		Emitter:     output.New(output.ModeJSON),
	})
	if err != nil {
		t.Fatalf("%v checks=%+v", err, checks)
	}
}

func TestDoctorFlagsSyncCerts(t *testing.T) {
	if _, err := exec.LookPath("flutter"); err != nil {
		t.Skip("flutter not on PATH")
	}
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte("dependencies:\n  flutter:\n    sdk: flutter\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, ".metadata"), []byte("v:1\n"), 0o644)
	app := filepath.Join(dir, "android", "app")
	_ = os.MkdirAll(app, 0o755)
	_ = os.WriteFile(filepath.Join(app, "build.gradle"), []byte(`
key.properties
android { applicationId "com.example.app"
  signingConfigs { release {} }
}
`), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "Ternfile"), []byte("lane r:\n  sync_certs pull repo:env:CERT_REPO\n"), 0o644)
	t.Setenv("CERT_REPO", "git@example.com:org/certs.git")
	cfg, _ := config.Load(dir)
	_, err := doctor.Run(doctor.Options{
		ProjectRoot: dir,
		Config:      cfg,
		Registry:    adapter.NewRegistry(flutter.New(nil)),
		Emitter:     output.New(output.ModeJSON),
	})
	if err == nil {
		t.Fatal("expected sync_certs doctor failure")
	}
}
