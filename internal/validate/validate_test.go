package validate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/darkmintis/Tern/internal/errors"
)

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func flutterRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, "pubspec.yaml", "name: demo\nversion: 1.2.3+4\n")
	writeFile(t, dir, "android/app/build.gradle", `android: {
  namespace "com.example.demo"
}`)
	return dir
}

func aab(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, "app-release.aab")
	writeFile(t, dir, filepath.Base(p), "aab-bytes")
	return p
}

func ipa(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, "app.ipa")
	writeFile(t, dir, filepath.Base(p), "ipa-bytes")
	return p
}

func checkByName(t *testing.T, res Result, name string) *Check {
	t.Helper()
	for i := range res.Checks {
		if res.Checks[i].Name == name {
			return &res.Checks[i]
		}
	}
	t.Fatalf("no check %q in %+v", name, res.Checks)
	return nil
}

func TestRunMissingPubspec(t *testing.T) {
	_, err := Run(Options{ProjectRoot: t.TempDir(), Target: "play_store"})
	if err == nil {
		t.Fatal("expected failure for missing pubspec")
	}
	if class, ok := ternerrors.AsClass(err); !ok || class != ternerrors.ClassUpload {
		t.Fatalf("class=%q", class)
	}
}

func TestRunPlayStorePass(t *testing.T) {
	dir := flutterRoot(t)
	art := aab(t, dir)
	creds := filepath.Join(dir, "creds.json")
	writeFile(t, dir, filepath.Base(creds), `{"type":"service_account"}`)
	t.Setenv("ANDROID_PACKAGE_NAME", "com.example.demo")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", creds)

	res, err := Run(Options{ProjectRoot: dir, Artifact: art, Target: "play_store"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("result: %+v", res)
	}
	for _, name := range []string{"version", "artifact", "extension", "play_credentials", "package_id", "version_match"} {
		if c := checkByName(t, res, name); !c.OK {
			t.Fatalf("check %s failed: %+v", name, c)
		}
	}
}

func TestRunPlayStoreMissingCredentials(t *testing.T) {
	dir := flutterRoot(t)
	art := aab(t, dir)
	t.Setenv("ANDROID_PACKAGE_NAME", "com.example.demo")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	res, err := Run(Options{ProjectRoot: dir, Artifact: art, Target: "play_store"})
	if err == nil {
		t.Fatal("expected failure without credentials")
	}
	if res.OK {
		t.Fatal("must not pass")
	}
	if c := checkByName(t, res, "play_credentials"); c.OK {
		t.Fatalf("play_credentials should fail: %+v", c)
	}
}

func TestRunPlayStoreWrongExtension(t *testing.T) {
	dir := flutterRoot(t)
	art := ipa(t, dir)
	t.Setenv("ANDROID_PACKAGE_NAME", "com.example.demo")
	res, err := Run(Options{ProjectRoot: dir, Artifact: art, Target: "play_store"})
	if err == nil {
		t.Fatal("expected extension failure")
	}
	if c := checkByName(t, res, "extension"); c.OK {
		t.Fatalf("extension should fail: %+v", c)
	}
}

func TestRunAppStoreMissingEnv(t *testing.T) {
	dir := flutterRoot(t)
	art := ipa(t, dir)
	t.Setenv("IOS_BUNDLE_ID", "com.example.demo")
	for _, k := range []string{
		"APP_STORE_CONNECT_API_KEY_ID",
		"APP_STORE_CONNECT_API_ISSUER_ID",
		"APP_STORE_CONNECT_API_KEY_PATH",
	} {
		t.Setenv(k, "")
	}
	res, err := Run(Options{ProjectRoot: dir, Artifact: art, Target: "testflight"})
	if err == nil {
		t.Fatal("expected failure with missing ASC env")
	}
	if res.OK {
		t.Fatal("must not pass")
	}
	for _, name := range []string{"asc_app_store_connect_api_key_id", "asc_app_store_connect_api_issuer_id", "asc_app_store_connect_api_key_path"} {
		if c := checkByName(t, res, name); c.OK {
			t.Fatalf("%s should fail", name)
		}
	}
}

func TestRunAppStorePass(t *testing.T) {
	dir := flutterRoot(t)
	art := ipa(t, dir)
	t.Setenv("IOS_BUNDLE_ID", "com.example.demo")
	keyPath := filepath.Join(dir, "AuthKey.p8")
	writeFile(t, dir, filepath.Base(keyPath), "key")
	t.Setenv("APP_STORE_CONNECT_API_KEY_ID", "ABC123")
	t.Setenv("APP_STORE_CONNECT_API_ISSUER_ID", "ISSUER")
	t.Setenv("APP_STORE_CONNECT_API_KEY_PATH", keyPath)

	res, err := Run(Options{ProjectRoot: dir, Artifact: art, Target: "app_store"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("result: %+v", res)
	}
	for _, name := range []string{"bundle_id", "extension", "asc_app_store_connect_api_key_id"} {
		if c := checkByName(t, res, name); !c.OK {
			t.Fatalf("check %s failed: %+v", name, c)
		}
	}
}

func TestRunForce(t *testing.T) {
	dir := flutterRoot(t)
	art := aab(t, dir)
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	res, err := Run(Options{ProjectRoot: dir, Artifact: art, Target: "play_store", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatal("--force must force OK")
	}
	if res.Message == "" {
		t.Fatal("expected force message")
	}
}

func TestRunDefaultsToLastArtifact(t *testing.T) {
	dir := flutterRoot(t)
	// No saved artifact -> artifact check fails but no panic.
	t.Setenv("ANDROID_PACKAGE_NAME", "com.example.demo")
	res, err := Run(Options{ProjectRoot: dir, Target: ""})
	if err == nil {
		t.Fatal("expected failure")
	}
	if c := checkByName(t, res, "artifact"); c.OK {
		t.Fatalf("artifact should fail without saved record: %+v", c)
	}
}
