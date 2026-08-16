package cache

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGHAFragmentContent(t *testing.T) {
	for _, want := range []string{
		"actions/cache@v4",
		"tern-pub-",
		"tern-gradle-",
		"Cache CocoaPods",
		"tern-pods-",
	} {
		if !strings.Contains(GHAFragment, want) {
			t.Fatalf("fragment missing %q", want)
		}
	}
}

func TestWriteGHAFragmentStdout(t *testing.T) {
	for _, path := range []string{"", "-"} {
		msg, err := WriteGHAFragment(path)
		if err != nil {
			t.Fatalf("WriteGHAFragment(%q): %v", path, err)
		}
		if msg != GHAFragment {
			t.Fatalf("stdout message mismatch for %q", path)
		}
	}
}

func TestWriteGHAFragmentFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "tern-cache.yml")
	msg, err := WriteGHAFragment(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "wrote " + path; msg != want {
		t.Fatalf("msg=%q want %q", msg, want)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != GHAFragment {
		t.Fatal("file content mismatch")
	}
}

func TestExplain(t *testing.T) {
	got := Explain()
	for _, want := range []string{"pub-cache", "Gradle", "CocoaPods", "tern cache --github-actions"} {
		if !strings.Contains(got, want) {
			t.Fatalf("Explain missing %q in %q", want, got)
		}
	}
}
