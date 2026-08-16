package fingerprint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/darkmintis/Tern/internal/config"
)

func writeRootFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func flutterProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeRootFile(t, dir, "pubspec.yaml", "name: demo\nversion: 1.0.0+1\n")
	writeRootFile(t, dir, "pubspec.lock", "pkgs:\n  flutter: v1\n")
	writeRootFile(t, dir, "lib/main.dart", "void main() {}\n")
	return dir
}

func TestComputeStable(t *testing.T) {
	dir := flutterProject(t)
	a, err := Compute(Input{ProjectRoot: dir, Platform: config.PlatformAndroid, Mode: config.ModeRelease})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Compute(Input{ProjectRoot: dir, Platform: config.PlatformAndroid, Mode: config.ModeRelease})
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("expected stable hash, got %q vs %q", a, b)
	}
}

func TestComputeChangesOnSource(t *testing.T) {
	dir := flutterProject(t)
	base, err := Compute(Input{ProjectRoot: dir, Platform: config.PlatformAndroid, Mode: config.ModeRelease})
	if err != nil {
		t.Fatal(err)
	}
	writeRootFile(t, dir, "lib/main.dart", "void main() { changed(); }\n")
	changed, err := Compute(Input{ProjectRoot: dir, Platform: config.PlatformAndroid, Mode: config.ModeRelease})
	if err != nil {
		t.Fatal(err)
	}
	if base == changed {
		t.Fatal("source change must change fingerprint")
	}
}

func TestComputePlatformSpecific(t *testing.T) {
	dir := flutterProject(t)
	android, _ := Compute(Input{ProjectRoot: dir, Platform: config.PlatformAndroid, Mode: config.ModeRelease})
	ios, _ := Compute(Input{ProjectRoot: dir, Platform: config.PlatformIOS, Mode: config.ModeRelease})
	if android == ios {
		t.Fatal("android and ios fingerprints must differ")
	}
}

func TestComputeInputTrips(t *testing.T) {
	dir := flutterProject(t)
	picks := []string{"kind", "flavor", "scheme", "mode"}
	inputs := map[string]Input{
		"kind":   {ProjectRoot: dir, Platform: config.PlatformAndroid, Mode: config.ModeRelease, Kind: "aab"},
		"flavor": {ProjectRoot: dir, Platform: config.PlatformAndroid, Mode: config.ModeRelease, Flavor: "free"},
		"scheme": {ProjectRoot: dir, Platform: config.PlatformAndroid, Mode: config.ModeRelease, Scheme: "Release"},
		"mode":   {ProjectRoot: dir, Platform: config.PlatformAndroid, Mode: config.ModeDebug},
	}
	base, _ := Compute(Input{ProjectRoot: dir, Platform: config.PlatformAndroid, Mode: config.ModeRelease})
	for _, k := range picks {
		h, err := Compute(inputs[k])
		if err != nil {
			t.Fatal(err)
		}
		if h == base {
			t.Fatalf("%s override must change fingerprint", k)
		}
	}
}

func TestComputeIgnoresBuildNoise(t *testing.T) {
	dir := flutterProject(t)
	base, _ := Compute(Input{ProjectRoot: dir, Platform: config.PlatformAndroid, Mode: config.ModeRelease})
	writeRootFile(t, dir, "build/app/outputs/app-release.aab", "binary\n")
	writeRootFile(t, dir, ".dart_tool/package_config.json", "noise\n")
	writeRootFile(t, dir, "android/app/build/intermediates/junk", "noise\n")
	after, _ := Compute(Input{ProjectRoot: dir, Platform: config.PlatformAndroid, Mode: config.ModeRelease})
	if base != after {
		t.Fatal("build/ and .dart_tool noise must be excluded")
	}
}

func TestLockfiles(t *testing.T) {
	dir := t.TempDir()
	got := Lockfiles(dir)
	if len(got) != 3 {
		t.Fatalf("expected 3 lockfiles, got %d", len(got))
	}
}

func TestLockfileHash(t *testing.T) {
	dir := t.TempDir()
	writeRootFile(t, dir, "pubspec.lock", "pkgs:\n  a: v1\n")
	a, err := LockfileHash(dir)
	if err != nil {
		t.Fatal(err)
	}
	writeRootFile(t, dir, "pubspec.lock", "pkgs:\n  a: v2\n")
	b, err := LockfileHash(dir)
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("changed lockfile must change hash")
	}
	// Missing lockfiles produce a deterministic empty hash, no error.
	empty, err := LockfileHash(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if empty == "" || empty == a {
		t.Fatal("unexpected empty-lockfile hash")
	}
}
