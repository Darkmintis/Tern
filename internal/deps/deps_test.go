package deps

import (
	"os"
	"path/filepath"
	"testing"
)

func writeLock(t *testing.T, root, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "pubspec.lock"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestShouldSkipPubGetNoState(t *testing.T) {
	dir := t.TempDir()
	writeLock(t, dir, "pkgs:\n")
	skip, sum, err := ShouldSkipPubGet(dir)
	if err != nil {
		t.Fatal(err)
	}
	if skip {
		t.Fatal("no state file must not skip")
	}
	if sum == "" {
		t.Fatal("expected a fingerprint sum")
	}
}

func TestSkipAfterMarkResolved(t *testing.T) {
	dir := t.TempDir()
	writeLock(t, dir, "pkgs:\n")
	if err := MarkResolved(dir, ""); err != nil {
		t.Fatal(err)
	}
	skip, _, err := ShouldSkipPubGet(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !skip {
		t.Fatal("expected skip after MarkResolved")
	}
}

func TestSkipClearedWhenLockChanges(t *testing.T) {
	dir := t.TempDir()
	writeLock(t, dir, "pkgs:\n  a: v1\n")
	if err := MarkResolved(dir, ""); err != nil {
		t.Fatal(err)
	}
	writeLock(t, dir, "pkgs:\n  a: v2\n")
	skip, _, err := ShouldSkipPubGet(dir)
	if err != nil {
		t.Fatal(err)
	}
	if skip {
		t.Fatal("changed lockfile must invalidate skip")
	}
}

func TestMarkResolvedWithExplicitSum(t *testing.T) {
	dir := t.TempDir()
	if err := MarkResolved(dir, "fixed-sum"); err != nil {
		t.Fatal(err)
	}
	skip, _, err := ShouldSkipPubGet(dir)
	if err != nil {
		t.Fatal(err)
	}
	if skip {
		t.Fatal("no lockfiles means fingerprints never match; must not skip")
	}
}

func TestMissingLockfileNoError(t *testing.T) {
	dir := t.TempDir()
	skip, sum, err := ShouldSkipPubGet(dir)
	if err != nil {
		t.Fatal(err)
	}
	if skip {
		t.Fatal("missing lockfile must not skip")
	}
	if sum == "" {
		t.Fatal("expected empty-lockfile hash")
	}
}
