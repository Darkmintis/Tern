package reactnative_test

import (
	"testing"

	"github.com/darkmintis/Tern/internal/adapter/reactnative"
)

func TestScaffoldOnly(t *testing.T) {
	if reactnative.New(nil).Detect(t.TempDir()) {
		t.Fatal("Detect must stay off in v0")
	}
	if reactnative.Phase != 4 {
		t.Fatalf("phase: %d", reactnative.Phase)
	}
}
