package signing_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/darkmintis/Tern/internal/config"
	ternerrors "github.com/darkmintis/Tern/internal/errors"
	"github.com/darkmintis/Tern/internal/signing"
)

func TestWriteAndroidKeyProperties(t *testing.T) {
	dir := t.TempDir()
	ks := filepath.Join(dir, "upload.jks")
	_ = os.WriteFile(ks, []byte("fake"), 0o600)
	t.Setenv("ANDROID_KEYSTORE", ks)
	t.Setenv("ANDROID_KEYSTORE_PASSWORD", "store-pass-strong")
	t.Setenv("ANDROID_KEY_ALIAS", "upload")
	t.Setenv("ANDROID_KEY_PASSWORD", "key-pass-strong")

	out, err := signing.WriteAndroidKeyProperties(dir, "ANDROID_KEYSTORE")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{"storePassword=", "keyAlias=upload", "storeFile="} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in %s", want, s)
		}
	}
}

func TestSignAndroidDryRun(t *testing.T) {
	dir := t.TempDir()
	ks := filepath.Join(dir, "upload.jks")
	_ = os.WriteFile(ks, []byte("fake"), 0o600)
	t.Setenv("ANDROID_KEYSTORE", ks)
	t.Setenv("ANDROID_KEYSTORE_PASSWORD", "store-pass-strong")
	t.Setenv("ANDROID_KEY_ALIAS", "upload")
	t.Setenv("ANDROID_KEY_PASSWORD", "key-pass-strong")

	m := signing.NewManager()
	res, err := m.Sign(t.Context(), signing.SignOptions{
		Platform:    config.PlatformAndroid,
		With:        "keystore",
		EnvRef:      "ANDROID_KEYSTORE",
		ProjectRoot: dir,
		DryRun:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Message, "dry-run") {
		t.Fatalf("%s", res.Message)
	}
}

func setupAndroidEnv(t *testing.T, dir string) {
	t.Helper()
	ks := filepath.Join(dir, "upload.jks")
	if err := os.WriteFile(ks, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ANDROID_KEYSTORE", ks)
	t.Setenv("ANDROID_KEYSTORE_PASSWORD", "store-pass-strong")
	t.Setenv("ANDROID_KEY_ALIAS", "upload")
	t.Setenv("ANDROID_KEY_PASSWORD", "key-pass-strong")
}

func TestSignAndroidLiveWritesKeyProperties(t *testing.T) {
	dir := t.TempDir()
	setupAndroidEnv(t, dir)
	m := signing.NewManager()
	res, err := m.Sign(t.Context(), signing.SignOptions{
		Platform:    config.PlatformAndroid,
		With:        "keystore",
		EnvRef:      "ANDROID_KEYSTORE",
		ProjectRoot: dir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Message, "key.properties") {
		t.Fatalf("%s", res.Message)
	}
	if _, err := os.Stat(filepath.Join(dir, "android", "key.properties")); err != nil {
		t.Fatalf("key.properties not written: %v", err)
	}
}

func TestSignIOSValidated(t *testing.T) {
	m := signing.NewManager()
	t.Setenv("IOS_CERT", "https://example.com/cert.p12")
	res, err := m.Sign(t.Context(), signing.SignOptions{
		Platform: config.PlatformIOS,
		With:     "cert",
		EnvRef:   "IOS_CERT",
		DryRun:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Message, "ios") {
		t.Fatalf("%s", res.Message)
	}
}

func TestSignUnsupportedPlatform(t *testing.T) {
	m := signing.NewManager()
	t.Setenv("MAC_CERT", "strong-placeholder-path-ok")
	_, err := m.Sign(t.Context(), signing.SignOptions{
		Platform: config.Platform("macos"),
		With:     "cert",
		EnvRef:   "MAC_CERT",
	})
	if err == nil {
		t.Fatal("expected unsupported platform error")
	}
	if class, ok := ternerrors.AsClass(err); !ok || class != ternerrors.ClassSign {
		t.Fatalf("class=%q", class)
	}
}

func TestSignWeakSecretRejected(t *testing.T) {
	m := signing.NewManager()
	t.Setenv("ANDROID_KEYSTORE", "password")
	res, err := m.Sign(t.Context(), signing.SignOptions{
		Platform: config.PlatformAndroid,
		With:     "keystore",
		EnvRef:   "ANDROID_KEYSTORE",
		DryRun:   true,
	})
	if err == nil {
		t.Fatalf("weak secret must fail, got %+v", res)
	}
	if class, ok := ternerrors.AsClass(err); !ok || class != ternerrors.ClassDoctor {
		t.Fatalf("class=%q", class)
	}
}

func TestSignMissingKeystoreFile(t *testing.T) {
	m := signing.NewManager()
	t.Setenv("ANDROID_KEYSTORE", filepath.Join(t.TempDir(), "missing.jks"))
	t.Setenv("ANDROID_KEYSTORE_PASSWORD", "store-pass-strong")
	t.Setenv("ANDROID_KEY_ALIAS", "upload")
	t.Setenv("ANDROID_KEY_PASSWORD", "key-pass-strong")
	_, err := m.Sign(t.Context(), signing.SignOptions{
		Platform:    config.PlatformAndroid,
		With:        "keystore",
		EnvRef:      "ANDROID_KEYSTORE",
		ProjectRoot: t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected missing keystore error")
	}
	if hint := ternerrors.HintOf(err); hint == "" {
		t.Fatalf("expected hint, got %v", err)
	}
}

func TestSignIncompleteAndroidEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ANDROID_KEYSTORE", filepath.Join(dir, "upload.jks"))
	t.Setenv("ANDROID_KEYSTORE_PASSWORD", "store-pass-strong")
	t.Setenv("ANDROID_KEY_ALIAS", "")
	t.Setenv("ANDROID_KEY_PASSWORD", "")
	m := signing.NewManager()
	_, err := m.Sign(t.Context(), signing.SignOptions{
		Platform:    config.PlatformAndroid,
		With:        "keystore",
		EnvRef:      "ANDROID_KEYSTORE",
		ProjectRoot: dir,
		DryRun:      true,
	})
	if err == nil {
		t.Fatal("expected incomplete env error")
	}
}

func TestSignResolvesUnsetEnv(t *testing.T) {
	t.Setenv("ANDROID_KEYSTORE", "")
	m := signing.NewManager()
	_, err := m.Sign(t.Context(), signing.SignOptions{
		Platform:    config.PlatformAndroid,
		With:        "keystore",
		EnvRef:      "ANDROID_KEYSTORE",
		ProjectRoot: t.TempDir(),
		DryRun:      true,
	})
	if err == nil {
		t.Fatal("expected unset env error")
	}
}

func TestWriteAndroidKeyPropertiesRequiresRoot(t *testing.T) {
	_, err := signing.WriteAndroidKeyProperties("", "ANDROID_KEYSTORE")
	if err == nil {
		t.Fatal("expected project root error")
	}
}

func TestCheckProfileExpiry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Profile.mobileprovision")
	if err := os.WriteFile(path, []byte("p"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := signing.CheckProfileExpiry(path); err != nil {
		t.Fatalf("fresh profile must pass: %v", err)
	}
}

func TestCheckProfileExpiryStale(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Profile.mobileprovision")
	if err := os.WriteFile(path, []byte("p"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-366 * 24 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	if err := signing.CheckProfileExpiry(path); err == nil {
		t.Fatal("stale profile must fail")
	}
}

func TestCheckProfileExpiryMissing(t *testing.T) {
	if err := signing.CheckProfileExpiry(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("missing profile must fail")
	}
}

func TestWriteAndroidKeyPropertiesMissingKeystoreEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ANDROID_KEYSTORE", "")
	_, err := signing.WriteAndroidKeyProperties(dir, "ANDROID_KEYSTORE")
	if err == nil {
		t.Fatal("expected error for empty keystore env")
	}
}

func TestCertSyncDryRun(t *testing.T) {
	c := &signing.CertSync{Backend: nil}
	t.Setenv("CERT_REPO", "git@example.com:org/certs.git")
	msg, err := c.Sync(t.Context(), signing.SyncOptions{Action: "pull", RepoEnv: "CERT_REPO", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "dry-run") {
		t.Fatalf("%s", msg)
	}
}

func TestCertSyncPullPush(t *testing.T) {
	backend := &signing.NoopBackend{}
	c := &signing.CertSync{Backend: backend}
	root := t.TempDir()
	for action, want := range map[string]string{"pull": "certs pulled", "push": "certs pushed"} {
		msg, err := c.Sync(t.Context(), signing.SyncOptions{Action: action, LocalDir: root})
		if err != nil {
			t.Fatalf("%s: %v", action, err)
		}
		if msg != want {
			t.Fatalf("%s: msg=%q want %q", action, msg, want)
		}
	}
}

func TestCertSyncUnknownAction(t *testing.T) {
	c := &signing.CertSync{Backend: &signing.NoopBackend{}}
	_, err := c.Sync(t.Context(), signing.SyncOptions{Action: "clone", LocalDir: t.TempDir()})
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
	if class, ok := ternerrors.AsClass(err); !ok || class != ternerrors.ClassSign {
		t.Fatalf("class=%q", class)
	}
}

func TestCertSyncUnsetRepoEnv(t *testing.T) {
	t.Setenv("CERT_REPO", "")
	c := &signing.CertSync{Backend: &signing.NoopBackend{}}
	_, err := c.Sync(t.Context(), signing.SyncOptions{Action: "pull", RepoEnv: "CERT_REPO", LocalDir: t.TempDir()})
	if err == nil {
		t.Fatal("expected missing repo env error")
	}
}
