package engine

import (
	"testing"

	"github.com/darkmintis/Tern/internal/config"
)

func TestPlayTracksFromLane(t *testing.T) {
	lane := config.Lane{Steps: []config.Step{
		{Kind: config.StepBump},
		{Kind: config.StepUpload, UploadTarget: "play_store", Track: "internal"},
		{Kind: config.StepUpload, UploadTarget: "play_store", Track: "internal"},
		{Kind: config.StepUpload, UploadTarget: "testflight"},
		{Kind: config.StepShip, UploadTarget: "play_store", Track: "production"},
	}}
	got := playTracksFromLane(lane)
	if len(got) != 2 {
		t.Fatalf("got %#v", got)
	}
	if got[0].Track != "internal" || got[1].Track != "production" {
		t.Fatalf("got %#v", got)
	}
}
