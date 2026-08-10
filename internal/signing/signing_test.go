package signing_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/darkmintis/Tern/internal/config"
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
