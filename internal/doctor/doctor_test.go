package doctor_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/darkmintis/Tern/internal/adapter"
	"github.com/darkmintis/Tern/internal/adapter/flutter"
	"github.com/darkmintis/Tern/internal/config"
	"github.com/darkmintis/Tern/internal/doctor"
	"github.com/darkmintis/Tern/internal/output"
)

// noopRunner never shells out, so unit tests are hermetic regardless of
// whether real toolchains are on PATH (AGENTS.md).
type noopRunner struct{}

func (noopRunner) Run(ctx context.Context, dir, name string, args ...string) (string, error) {
	return "", nil
}

// testFlutterAdapter returns a Flutter adapter whose PATH lookup and command
// execution are both stubbed, so doctor.Run never touches a real flutter.
func testFlutterAdapter() *flutter.Adapter {
	ad := flutter.New(noopRunner{})
	ad.LookPath = func(string) (string, error) { return "/ok/flutter", nil }
	return ad
}

func TestDoctorFlutterAndroid(t *testing.T) {
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
	sdk := filepath.Join(dir, "android-sdk")
	_ = os.MkdirAll(sdk, 0o755)
	t.Setenv("ANDROID_HOME", sdk)
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
		Registry:    adapter.NewRegistry(testFlutterAdapter()),
		Emitter:     output.New(output.ModeJSON),
	})
	if err != nil {
		t.Fatalf("%v checks=%+v", err, checks)
	}
	assertCheck(t, checks, "adapter", true)
	assertCheck(t, checks, "android_signing_gradle", true)
	assertCheck(t, checks, "android_package", true)
	assertCheck(t, checks, "env:ANDROID_KEYSTORE", true)
}

func TestDoctorFlagsSyncCerts(t *testing.T) {
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
	sdk := filepath.Join(dir, "android-sdk")
	_ = os.MkdirAll(sdk, 0o755)
	t.Setenv("ANDROID_HOME", sdk)
	t.Setenv("CERT_REPO", "git@example.com:org/certs.git")
	cfg, _ := config.Load(dir)
	checks, err := doctor.Run(doctor.Options{
		ProjectRoot: dir,
		Config:      cfg,
		Registry:    adapter.NewRegistry(testFlutterAdapter()),
		Emitter:     output.New(output.ModeJSON),
	})
	if err == nil {
		t.Fatal("expected sync_certs doctor failure")
	}
	assertCheck(t, checks, "sync_certs", false)
}

// assertCheck finds a named doctor check and verifies its OK flag.
func assertCheck(t *testing.T, checks []doctor.Check, name string, ok bool) {
	t.Helper()
	for _, c := range checks {
		if c.Name == name {
			if c.OK != ok {
				t.Fatalf("check %q: OK=%v want %v (%s)", name, c.OK, ok, c.Message)
			}
			return
		}
	}
	t.Fatalf("no check named %q; got %d checks", name, len(checks))
}
