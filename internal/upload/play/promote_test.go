package play

import (
	"context"
	"strings"
	"testing"

	ternerrors "github.com/darkmintis/Tern/internal/errors"
	"google.golang.org/api/androidpublisher/v3"
)

func TestMaxVersionCode(t *testing.T) {
	if got := maxVersionCode([]int64{1, 9, 4}); got != 9 {
		t.Fatalf("got %d", got)
	}
	if got := maxVersionCode(nil); got != 0 {
		t.Fatalf("got %d", got)
	}
}

func TestNewestEligible(t *testing.T) {
	cases := []struct {
		name     string
		releases []*androidpublisher.TrackRelease
		wantVC   int64
	}{
		{"nil", nil, 0},
		{"skips draft", []*androidpublisher.TrackRelease{
			{Status: "draft", VersionCodes: []int64{99}},
			{Status: "completed", VersionCodes: []int64{5}},
		}, 5},
		{"picks completed over inProgress", []*androidpublisher.TrackRelease{
			{Status: "inProgress", VersionCodes: []int64{8}},
			{Status: "completed", VersionCodes: []int64{7}},
		}, 8},
		{"underscore status tolerated", []*androidpublisher.TrackRelease{
			{Status: "in_progress", VersionCodes: []int64{6}},
		}, 6},
		{"only drafts", []*androidpublisher.TrackRelease{
			{Status: "draft", VersionCodes: []int64{3}},
		}, 0},
		{"nil release skipped", []*androidpublisher.TrackRelease{nil, {Status: "completed", VersionCodes: []int64{2}}}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := newestEligible(tc.releases)
			if tc.wantVC == 0 {
				if got != nil {
					t.Fatalf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil || maxVersionCode(got.VersionCodes) != tc.wantVC {
				t.Fatalf("got %+v want vc %d", got, tc.wantVC)
			}
		})
	}
}

func TestBuildTrackUpdate(t *testing.T) {
	rel := SourceRelease{Eligible: true, VersionCode: 42, Name: "1.2.3 (42)"}

	done := buildTrackUpdate(PromoteRequest{TargetTrack: "production", Release: rel})
	if done.Track != "production" || len(done.Releases) != 1 {
		t.Fatalf("track=%+v", done)
	}
	r := done.Releases[0]
	if r.Status != "completed" || r.VersionCodes[0] != 42 || r.Name != "1.2.3 (42)" {
		t.Fatalf("release=%+v", r)
	}

	staged := buildTrackUpdate(PromoteRequest{TargetTrack: "beta", Release: rel, UserFraction: 0.1})
	if staged.Releases[0].Status != "inProgress" || staged.Releases[0].UserFraction != 0.1 {
		t.Fatalf("staged release=%+v", staged.Releases[0])
	}
}

func TestPromoteValidationBranches(t *testing.T) {
	cases := []struct {
		name string
		req  PromoteRequest
		want string
	}{
		{"empty package", PromoteRequest{TargetTrack: "production", Release: SourceRelease{Eligible: true, VersionCode: 1}}, "empty package name"},
		{"empty target", PromoteRequest{PackageName: "com.x", Release: SourceRelease{Eligible: true, VersionCode: 1}}, "empty target track"},
		{"ineligible release", PromoteRequest{PackageName: "com.x", TargetTrack: "production", Release: SourceRelease{}}, "no eligible source release"},
		{"zero version code", PromoteRequest{PackageName: "com.x", TargetTrack: "production", Release: SourceRelease{Eligible: true}}, "no eligible source release"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := (APIClient{}).Promote(context.Background(), tc.req)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v want contains %q", err, tc.want)
			}
			if class, ok := ternerrors.AsClass(err); !ok || class != ternerrors.ClassUpload {
				t.Fatalf("class=%q", class)
			}
		})
	}
}

func TestLookupRequiresPackage(t *testing.T) {
	_, err := (APIClient{}).Lookup(context.Background(), LookupRequest{})
	if err == nil || !strings.Contains(err.Error(), "empty package name") {
		t.Fatalf("got %v", err)
	}
}
