package version

import "testing"

func TestVersionSet(t *testing.T) {
	if Version == "" {
		t.Fatal("Version must never be empty (set via -ldflags at release)")
	}
}
