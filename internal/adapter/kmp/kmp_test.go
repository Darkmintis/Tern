package kmp_test

import (
	"testing"

	"github.com/darkmintis/Tern/internal/adapter/kmp"
)

func TestScaffoldOnly(t *testing.T) {
	if kmp.New(nil).Detect(t.TempDir()) {
		t.Fatal("Detect must stay off in v0")
	}
	if kmp.Phase != 3 {
		t.Fatalf("phase: %d", kmp.Phase)
	}
}
