package config_test

import (
	"strings"
	"testing"

	"github.com/darkmintis/Tern/internal/config"
)

// FuzzParseDSL must never panic on any input, and any output must round-trip
// into a well-formed Config. Input parsers are the highest-risk surface for a
// config-driven release tool.
func FuzzParseDSL(f *testing.F) {
	seeds := []string{
		"",
		"lane release:\n  build android release\n",
		"lane r:\n  sign android with keystore env:ANDROID_KEYSTORE\n  upload android to play_store track:internal\n",
		"lane ios:\n  ship ios from last to testflight release_name:version notes:text:x\n",
		"lane b:\n  bump version patch\n  tag v prefix:v\n  sync_certs pull repo:env:CERT_REPO\n  notify slack env:SLACK_WEBHOOK\n",
		"name: demo\nversion: 1.0.0+1\ndependencies:\n  flutter:\n    sdk: flutter\n",
		strings.Repeat("x", 4096),
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, src string) {
		if len(src) > 1<<16 {
			t.Skip()
		}
		cfg, err := config.ParseDSL(src)
		if err != nil {
			// A failed parse is fine; it must be a typed Config/other error.
			return
		}
		if cfg == nil {
			t.Fatal("nil Config without error")
		}
		for _, lane := range cfg.Lanes {
			for _, step := range lane.Steps {
				if step.Kind == "" {
					t.Fatalf("empty step kind in lane %q: %+v", lane.Name, step)
				}
			}
		}
	})
}

// FuzzParseYAML protects the Ternfile alias path used by load.go.
func FuzzParseYAML(f *testing.F) {
	seeds := []string{
		"",
		"lanes:\n  release:\n    - build android release\n",
		"lanes:\n  r:\n    - upload ios to testflight\n",
		"{not yaml [",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, src string) {
		if len(src) > 1<<16 {
			t.Skip()
		}
		cfg, err := config.ParseYAML([]byte(src))
		if err != nil {
			return
		}
		if cfg == nil {
			t.Fatal("nil Config without error")
		}
	})
}
