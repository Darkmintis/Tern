package config_test

import (
	"testing"

	"github.com/darkmintis/Tern/internal/config"
)

func TestParseDSL_ReleaseLane(t *testing.T) {
	src := `
# sample
lane release:
  build android release
  build ios release
  sign android with keystore env:ANDROID_KEYSTORE
  sign ios with cert env:IOS_CERT
  upload android to play_store track:internal
  upload ios to testflight

lane build_only:
  build android debug
`
	cfg, err := config.ParseDSL(src)
	if err != nil {
		t.Fatal(err)
	}
	rel, ok := cfg.Lane("release")
	if !ok {
		t.Fatal("missing release lane")
	}
	if len(rel.Steps) != 6 {
		t.Fatalf("want 6 steps, got %d", len(rel.Steps))
	}
	if rel.Steps[0].Kind != config.StepBuild || rel.Steps[0].Platform != config.PlatformAndroid {
		t.Fatalf("step0: %+v", rel.Steps[0])
	}
	if rel.Steps[2].EnvRef != "ANDROID_KEYSTORE" {
		t.Fatalf("sign env: %+v", rel.Steps[2])
	}
	if rel.Steps[4].Track != "internal" {
		t.Fatalf("track: %+v", rel.Steps[4])
	}
}

func TestParseDSL_UnknownStep(t *testing.T) {
	_, err := config.ParseDSL("lane x:\n  freestyle everything\n")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseDSL_Phase15Steps(t *testing.T) {
	src := `
lane release:
  sync_certs pull repo:env:CERT_REPO
  bump version patch
  tag git prefix:v
  notify slack env:SLACK_WEBHOOK
`
	cfg, err := config.ParseDSL(src)
	if err != nil {
		t.Fatal(err)
	}
	lane := cfg.Lanes["release"]
	if lane.Steps[0].Kind != config.StepSyncCerts || lane.Steps[0].EnvRef != "CERT_REPO" {
		t.Fatalf("sync: %+v", lane.Steps[0])
	}
	if lane.Steps[1].BumpLevel != config.BumpPatch {
		t.Fatalf("bump: %+v", lane.Steps[1])
	}
	if lane.Steps[2].TagPrefix != "v" {
		t.Fatalf("tag: %+v", lane.Steps[2])
	}
}

func TestParseYAML(t *testing.T) {
	data := []byte(`
lanes:
  release:
    - build:
        platform: android
        mode: release
    - upload:
        platform: android
        to: play_store
        track: internal
`)
	cfg, err := config.ParseYAML(data)
	if err != nil {
		t.Fatal(err)
	}
	lane := cfg.Lanes["release"]
	if len(lane.Steps) != 2 {
		t.Fatalf("steps: %d", len(lane.Steps))
	}
	if lane.Steps[1].Track != "internal" {
		t.Fatalf("%+v", lane.Steps[1])
	}
}

func TestParseDSL_TableDriven(t *testing.T) {
	cases := []struct {
		name    string
		line    string
		wantErr bool
		kind    config.StepKind
	}{
		{"build ok", "build ios debug", false, config.StepBuild},
		{"build bad mode", "build ios shipping", true, ""},
		{"sign ok", "sign ios with cert env:IOS_CERT", false, config.StepSign},
		{"sign no env", "sign ios with cert IOS_CERT", true, ""},
		{"upload ok", "upload ios to testflight", false, config.StepUpload},
		{"unknown", "deploy production", true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := "lane t:\n  " + tc.line + "\n"
			cfg, err := config.ParseDSL(src)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Lanes["t"].Steps[0].Kind != tc.kind {
				t.Fatalf("got %+v", cfg.Lanes["t"].Steps[0])
			}
		})
	}
}
