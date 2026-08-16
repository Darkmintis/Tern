package upload_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ternerrors "github.com/darkmintis/Tern/internal/errors"
	"github.com/darkmintis/Tern/internal/output"
	"github.com/darkmintis/Tern/internal/upload"
	"github.com/darkmintis/Tern/internal/upload/asc"
	"github.com/darkmintis/Tern/internal/upload/play"
)

// promoteFakePlay records promote calls and serves canned lookups.
type promoteFakePlay struct {
	lookups     []play.LookupRequest
	lastPromote *play.PromoteRequest
	lookupFn    func(play.LookupRequest) (play.SourceRelease, error)
	promoteFn   func(play.PromoteRequest) (string, error)
}

func (f *promoteFakePlay) Upload(context.Context, play.UploadRequest) (string, error) {
	return "unused", nil
}

func (f *promoteFakePlay) Lookup(_ context.Context, req play.LookupRequest) (play.SourceRelease, error) {
	f.lookups = append(f.lookups, req)
	if f.lookupFn != nil {
		return f.lookupFn(req)
	}
	return play.SourceRelease{Track: req.Track, Eligible: false}, nil
}

func (f *promoteFakePlay) Promote(_ context.Context, req play.PromoteRequest) (string, error) {
	if f.promoteFn != nil {
		return f.promoteFn(req)
	}
	f.lastPromote = &req
	return "promoted android", nil
}

type promoteFakeASC struct {
	lookups     int
	lastPromote *asc.PromoteRequest
	lookupFn    func() (asc.SourceBuild, error)
}

func (f *promoteFakeASC) Upload(context.Context, asc.UploadRequest) (string, error) {
	return "unused", nil
}

func (f *promoteFakeASC) Lookup(context.Context, asc.LookupRequest) (asc.SourceBuild, error) {
	f.lookups++
	if f.lookupFn != nil {
		return f.lookupFn()
	}
	return asc.SourceBuild{AppID: "APP1", BuildID: "BUILD9", BuildNumber: "17"}, nil
}

func (f *promoteFakeASC) Promote(_ context.Context, req asc.PromoteRequest) (string, error) {
	f.lastPromote = &req
	return "promoted ios", nil
}

func testEmitter(t *testing.T) (*bytes.Buffer, *output.Emitter) {
	t.Helper()
	var buf bytes.Buffer
	return &buf, &output.Emitter{Mode: output.ModeJSON, Out: &buf}
}

func androidPromoteClient(fp *promoteFakePlay) *upload.Client {
	return &upload.Client{Play: fp, ASC: &promoteFakeASC{}}
}

func TestParsePromoteTargets(t *testing.T) {
	cases := []struct {
		name           string
		source, target string
		wantPlatform   string
		wantSrc        string
		wantTgt        string
		wantErr        string
	}{
		{"internal to production", "internal", "production", "android", "internal", "production", ""},
		{"closed to production", "closed", "production", "android", "alpha", "production", ""},
		{"alpha to beta", "alpha", "beta", "android", "alpha", "beta", ""},
		{"prod alias", "prod", "beta", "android", "production", "beta", ""},
		{"testflight to appstore", "testflight", "appstore", "ios", "testflight", "appstore", ""},
		{"app_store spelling", "testflight", "app_store", "ios", "testflight", "appstore", ""},
		{"mixed platforms", "internal", "appstore", "", "", "", "must be both iOS stages"},
		{"unknown track", "internal", "gamma", "", "", "", "unknown Play track"},
		{"empty target", "internal", "", "", "", "", "empty source or target"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			platform, src, tgt, err := upload.ParsePromoteTargets(tc.source, tc.target)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("got %v want contains %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if string(platform) != tc.wantPlatform || src != tc.wantSrc || tgt != tc.wantTgt {
				t.Fatalf("got (%s,%s,%s) want (%s,%s,%s)", platform, src, tgt, tc.wantPlatform, tc.wantSrc, tc.wantTgt)
			}
		})
	}
}

func TestPromoteAndroidSuccess(t *testing.T) {
	buf, em := testEmitter(t)
	fp := &promoteFakePlay{}
	fp.lookupFn = func(req play.LookupRequest) (play.SourceRelease, error) {
		if req.Track == "production" {
			return play.SourceRelease{Track: "production", Eligible: false}, nil
		}
		return play.SourceRelease{Track: "internal", VersionCode: 42, Status: "completed", Name: "1.2.3 (42)", Eligible: true}, nil
	}
	var confirmedPlan upload.PromotePlan
	err := androidPromoteClient(fp).Promote(context.Background(), upload.PromoteOpts{
		ProjectRoot: t.TempDir(),
		Source:      "internal",
		Target:      "production",
		PackageName: "com.example.app",
		Rollout:     0.25,
		Emitter:     em,
		Confirm: func(plan upload.PromotePlan) (bool, error) {
			confirmedPlan = plan
			return true, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if fp.lastPromote == nil {
		t.Fatal("play.Promote not called")
	}
	if got := fp.lastPromote; got.TargetTrack != "production" || got.Release.VersionCode != 42 || got.UserFraction != 0.25 {
		t.Fatalf("promote req=%+v", got)
	}
	if confirmedPlan.Build != "42" || confirmedPlan.Track != "production" {
		t.Fatalf("plan=%+v", confirmedPlan)
	}
	if !strings.Contains(buf.String(), "promote_plan") || !strings.Contains(buf.String(), "staged rollout 25%") {
		t.Fatalf("events=%q", buf.String())
	}
}

func TestPromoteAndroidMissingSourceRelease(t *testing.T) {
	_, em := testEmitter(t)
	fp := &promoteFakePlay{}
	fp.lookupFn = func(req play.LookupRequest) (play.SourceRelease, error) {
		return play.SourceRelease{Track: req.Track, Eligible: false}, nil
	}
	var promoted bool
	err := androidPromoteClient(fp).Promote(context.Background(), upload.PromoteOpts{
		Source: "internal", Target: "production", PackageName: "com.example.app", Emitter: em,
		Confirm: func(upload.PromotePlan) (bool, error) { promoted = true; return true, nil },
	})
	if err == nil || !strings.Contains(err.Error(), "no eligible release on source track") {
		t.Fatalf("got %v", err)
	}
	if ternerrors.HintOf(err) == "" {
		t.Fatalf("missing hint: %v", err)
	}
	if fp.lastPromote != nil || promoted {
		t.Fatal("must not promote or confirm when source has no release")
	}
}

func TestPromoteAndroidTargetConflictWarns(t *testing.T) {
	fp := &promoteFakePlay{}
	fp.lookupFn = func(req play.LookupRequest) (play.SourceRelease, error) {
		switch req.Track {
		case "internal":
			return play.SourceRelease{Track: "internal", VersionCode: 42, Status: "completed", Eligible: true}, nil
		case "production":
			return play.SourceRelease{Track: "production", VersionCode: 90, Status: "completed", Eligible: true}, nil
		}
		return play.SourceRelease{Track: req.Track, Eligible: false}, nil
	}
	var gotPlan upload.PromotePlan
	err := androidPromoteClient(fp).Promote(context.Background(), upload.PromoteOpts{
		Source: "internal", Target: "production", PackageName: "com.example.app",
		Emitter: &output.Emitter{Mode: output.ModeJSON, Out: &bytes.Buffer{}},
		Confirm: func(plan upload.PromotePlan) (bool, error) {
			gotPlan = plan
			return true, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotPlan.Conflict, "newer release") || !strings.Contains(gotPlan.Conflict, "build 90") {
		t.Fatalf("conflict not surfaced: %+v", gotPlan)
	}
	if fp.lastPromote == nil {
		t.Fatal("expected promote after explicit confirmation of conflict")
	}
}

func TestPromoteAndroidConfirmationDeclinedAborts(t *testing.T) {
	buf, em := testEmitter(t)
	fp := &promoteFakePlay{}
	fp.lookupFn = func(req play.LookupRequest) (play.SourceRelease, error) {
		switch req.Track {
		case "internal":
			return play.SourceRelease{Track: "internal", VersionCode: 42, Status: "completed", Eligible: true}, nil
		}
		return play.SourceRelease{Track: req.Track, Eligible: false}, nil
	}
	err := androidPromoteClient(fp).Promote(context.Background(), upload.PromoteOpts{
		Source: "internal", Target: "production", PackageName: "com.example.app", Emitter: em,
		Confirm: func(upload.PromotePlan) (bool, error) { return false, nil },
	})
	if err == nil || !strings.Contains(err.Error(), "promotion canceled") {
		t.Fatalf("got %v", err)
	}
	if fp.lastPromote != nil {
		t.Fatal("must not promote when confirmation declined")
	}
	if !strings.Contains(buf.String(), "canceled") {
		t.Fatalf("expected canceled event, got %q", buf.String())
	}
}

func TestPromoteAndroidDryRunSkipsConfirmAndWrite(t *testing.T) {
	buf, em := testEmitter(t)
	fp := &promoteFakePlay{}
	fp.lookupFn = func(req play.LookupRequest) (play.SourceRelease, error) {
		switch req.Track {
		case "internal":
			return play.SourceRelease{Track: "internal", VersionCode: 42, Status: "completed", Eligible: true}, nil
		}
		return play.SourceRelease{Track: req.Track, Eligible: false}, nil
	}
	confirmed := false
	err := androidPromoteClient(fp).Promote(context.Background(), upload.PromoteOpts{
		Source: "internal", Target: "production", PackageName: "com.example.app",
		DryRun: true, Emitter: em,
		Confirm: func(upload.PromotePlan) (bool, error) { confirmed = true; return true, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if confirmed || fp.lastPromote != nil {
		t.Fatal("dry-run must not confirm or promote")
	}
	if !strings.Contains(buf.String(), "dry_run") {
		t.Fatalf("expected dry_run event, got %q", buf.String())
	}
}

func TestPromoteAndroidAlreadyLiveIsNoop(t *testing.T) {
	buf, em := testEmitter(t)
	fp := &promoteFakePlay{}
	fp.lookupFn = func(req play.LookupRequest) (play.SourceRelease, error) {
		return play.SourceRelease{Track: req.Track, VersionCode: 42, Status: "completed", Eligible: true}, nil
	}
	confirmed := false
	err := androidPromoteClient(fp).Promote(context.Background(), upload.PromoteOpts{
		Source: "internal", Target: "production", PackageName: "com.example.app", Emitter: em,
		Confirm: func(upload.PromotePlan) (bool, error) { confirmed = true; return true, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if confirmed || fp.lastPromote != nil {
		t.Fatal("idempotent promote must not confirm or write")
	}
	if !strings.Contains(buf.String(), "nothing to promote") {
		t.Fatalf("got %q", buf.String())
	}
}

func TestPromoteAndroidPackageFromProject(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "android", "app")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "build.gradle.kts"), []byte("android {\n    namespace = \"com.example.auto\"\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fp := &promoteFakePlay{}
	fp.lookupFn = func(req play.LookupRequest) (play.SourceRelease, error) {
		if req.PackageName != "com.example.auto" {
			t.Fatalf("package=%q", req.PackageName)
		}
		return play.SourceRelease{Track: req.Track, VersionCode: 42, Status: "completed", Eligible: true}, nil
	}
	err := androidPromoteClient(fp).Promote(context.Background(), upload.PromoteOpts{
		ProjectRoot: dir, Source: "internal", Target: "production",
		Emitter: &output.Emitter{Mode: output.ModeJSON, Out: &bytes.Buffer{}},
		Confirm: func(upload.PromotePlan) (bool, error) { return true, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPromoteAndroidRequiresPackage(t *testing.T) {
	err := androidPromoteClient(&promoteFakePlay{}).Promote(context.Background(), upload.PromoteOpts{
		Source: "internal", Target: "production", ProjectRoot: t.TempDir(),
	})
	if err == nil || !strings.Contains(err.Error(), "package name") {
		t.Fatalf("got %v", err)
	}
}

func TestPromoteIOSSuccess(t *testing.T) {
	buf, em := testEmitter(t)
	fa := &promoteFakeASC{}
	err := (&upload.Client{Play: &promoteFakePlay{}, ASC: fa}).Promote(context.Background(), upload.PromoteOpts{
		Source: "testflight", Target: "appstore", BundleID: "com.example.app",
		ReleaseVersion: "1.2.3", Emitter: em,
		Confirm: func(plan upload.PromotePlan) (bool, error) { return true, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if fa.lastPromote == nil {
		t.Fatal("asc.Promote not called")
	}
	req := fa.lastPromote
	if req.AppID != "APP1" || req.BuildID != "BUILD9" || req.BuildNumber != "17" || req.ReleaseVersion != "1.2.3" {
		t.Fatalf("promote req=%+v", req)
	}
	if !strings.Contains(buf.String(), "Apple review is still required") {
		t.Fatalf("events=%q", buf.String())
	}
}

func TestPromoteIOSRequiresReleaseVersion(t *testing.T) {
	dir := t.TempDir()
	err := (&upload.Client{Play: &promoteFakePlay{}, ASC: &promoteFakeASC{}}).Promote(context.Background(), upload.PromoteOpts{
		Source: "testflight", Target: "appstore", BundleID: "com.example.app",
		ProjectRoot: dir,
		Emitter:     &output.Emitter{Mode: output.ModeJSON, Out: &bytes.Buffer{}},
		Confirm:     func(upload.PromotePlan) (bool, error) { return true, nil },
	})
	if err == nil || !strings.Contains(err.Error(), "App Store version string required") {
		t.Fatalf("got %v", err)
	}
	if ternerrors.HintOf(err) == "" {
		t.Fatalf("missing hint: %v", err)
	}
}

func TestPromoteIOSVersionFromPubspec(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte("name: demo\nversion: 2.1.0+3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fa := &promoteFakeASC{}
	err := (&upload.Client{Play: &promoteFakePlay{}, ASC: fa}).Promote(context.Background(), upload.PromoteOpts{
		Source: "testflight", Target: "appstore", BundleID: "com.example.app",
		ProjectRoot: dir,
		Emitter:     &output.Emitter{Mode: output.ModeJSON, Out: &bytes.Buffer{}},
		Confirm:     func(upload.PromotePlan) (bool, error) { return true, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if fa.lastPromote.ReleaseVersion != "2.1.0" {
		t.Fatalf("release version=%q", fa.lastPromote.ReleaseVersion)
	}
}
