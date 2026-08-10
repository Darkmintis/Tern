package secrets_test

import (
	"testing"

	"github.com/darkmintis/Tern/internal/secrets"
)

func TestIsWeak(t *testing.T) {
	cases := []struct {
		in   string
		weak bool
	}{
		{"", true},
		{"changeme", true},
		{"SECRET", true},
		{"correct-horse-battery", false},
		{"a-real-keystore-password", false},
	}
	for _, tc := range cases {
		if got := secrets.IsWeak(tc.in); got != tc.weak {
			t.Fatalf("%q: got %v want %v", tc.in, got, tc.weak)
		}
	}
}

func TestCheckEnvStrong(t *testing.T) {
	t.Setenv("TERN_TEST_SECRET", "changeme")
	if err := secrets.CheckEnvStrong("TERN_TEST_SECRET"); err == nil {
		t.Fatal("expected weak error")
	}
	t.Setenv("TERN_TEST_SECRET", "strong-enough-value")
	if err := secrets.CheckEnvStrong("TERN_TEST_SECRET"); err != nil {
		t.Fatal(err)
	}
}
