package projectmeta_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/darkmintis/Tern/internal/projectmeta"
)

func writeMeta(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFlutterVersionErrors(t *testing.T) {
	if _, err := projectmeta.FlutterVersion(t.TempDir()); err == nil {
		t.Fatal("missing pubspec must error")
	}
	dir := t.TempDir()
	writeMeta(t, dir, "pubspec.yaml", "name: demo\n")
	if _, err := projectmeta.FlutterVersion(dir); err == nil {
		t.Fatal("pubspec without version must error")
	}
}

func TestAppDisplayName(t *testing.T) {
	dir := t.TempDir()
	writeMeta(t, dir, "pubspec.yaml", "name: my_super_app\n")
	cases := []struct {
		name    string
		envVar  string
		envVal  string
		project string
		want    string
	}{
		{"tern env wins", "TERN_APP_NAME", "Tern App", dir, "Tern App"},
		{"legacy env", "APP_DISPLAY_NAME", "Legacy", dir, "Legacy"},
		{"title-cased pubspec", "", "", dir, "My super app"},
		{"missing pubspec", "", "", t.TempDir(), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TERN_APP_NAME", tc.envVal)
			t.Setenv("APP_DISPLAY_NAME", tc.envVal)
			if tc.name == "title-cased pubspec" {
				t.Setenv("TERN_APP_NAME", "")
				t.Setenv("APP_DISPLAY_NAME", "")
			}
			got := projectmeta.AppDisplayName(tc.project)
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestIOSBundleIDEnvOverride(t *testing.T) {
	t.Setenv("IOS_BUNDLE_ID", "com.example.override")
	got, err := projectmeta.IOSBundleID(t.TempDir())
	if err != nil || got != "com.example.override" {
		t.Fatalf("%q %v", got, err)
	}
}

func TestIOSBundleIDPbxproj(t *testing.T) {
	dir := t.TempDir()
	writeMeta(t, dir, "ios/Runner.xcodeproj/project.pbxproj", `
PRODUCT_BUNDLE_IDENTIFIER = com.example.demo;
`)
	got, err := projectmeta.IOSBundleID(dir)
	if err != nil || got != "com.example.demo" {
		t.Fatalf("%q %v", got, err)
	}
}

func TestIOSBundleIDInfoPlist(t *testing.T) {
	dir := t.TempDir()
	writeMeta(t, dir, "ios/Runner/Info.plist", `
<key>CFBundleIdentifier</key>
<string>com.example.plist</string>
`)
	got, err := projectmeta.IOSBundleID(dir)
	if err != nil || got != "com.example.plist" {
		t.Fatalf("%q %v", got, err)
	}
}

func TestIOSBundleIDSkipsVarReferences(t *testing.T) {
	dir := t.TempDir()
	writeMeta(t, dir, "ios/Runner/Info.plist", `
<key>CFBundleIdentifier</key>
<string>$(PRODUCT_BUNDLE_IDENTIFIER)</string>
`)
	if _, err := projectmeta.IOSBundleID(dir); err == nil {
		t.Fatal("variable references must not be accepted")
	}
}

func TestIOSBundleIDUnresolved(t *testing.T) {
	dir := t.TempDir()
	writeMeta(t, dir, "ios/Runner.xcodeproj/project.pbxproj", `// no bundle id
`)
	_, err := projectmeta.IOSBundleID(dir)
	if err == nil || !strings.Contains(err.Error(), "could not detect iOS bundle id") {
		t.Fatalf("got %v", err)
	}
}

func TestAndroidPackageIDBuildGradleKts(t *testing.T) {
	dir := t.TempDir()
	writeMeta(t, dir, "android/app/build.gradle.kts", `
android {
    namespace = "com.example.kts"
}
`)
	got, err := projectmeta.AndroidPackageID(dir)
	if err != nil || got != "com.example.kts" {
		t.Fatalf("%q %v", got, err)
	}
}

func TestAndroidPackageIDUnresolved(t *testing.T) {
	dir := t.TempDir()
	_, err := projectmeta.AndroidPackageID(dir)
	if err == nil {
		t.Fatal("must error when no gradle file exists")
	}
}
