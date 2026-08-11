package safety

import "testing"

func TestIsProduction(t *testing.T) {
	cases := []struct {
		target, track string
		want          bool
	}{
		{"play_store", "internal", false},
		{"play_store", "beta", false},
		{"play_store", "production", true},
		{"play_store", "prod", true},
		{"play_store", "Production", true},
		{"app_store", "", true},
		{"testflight", "", false},
	}
	for _, tc := range cases {
		if got := IsProduction(tc.target, tc.track); got != tc.want {
			t.Fatalf("%s/%s: got %v want %v", tc.target, tc.track, got, tc.want)
		}
	}
}

func TestConfirmProduction_CIRequiresYes(t *testing.T) {
	ci, tty := true, false
	err := ConfirmProduction(ConfirmOpts{
		Target: "play_store",
		Track:  "production",
		IsCI:   &ci,
		IsTTY:  &tty,
	})
	if err == nil {
		t.Fatal("expected error without --yes in CI")
	}
	err = ConfirmProduction(ConfirmOpts{
		Target: "play_store",
		Track:  "production",
		Yes:    true,
		IsCI:   &ci,
		IsTTY:  &tty,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestConfirmProduction_Interactive(t *testing.T) {
	ci, tty := false, true
	err := ConfirmProduction(ConfirmOpts{
		Target: "play_store",
		Track:  "production",
		IsCI:   &ci,
		IsTTY:  &tty,
		Prompt: func(string) (string, error) { return "no", nil },
	})
	if err == nil {
		t.Fatal("expected cancel")
	}
	err = ConfirmProduction(ConfirmOpts{
		Target: "app_store",
		Track:  "",
		IsCI:   &ci,
		IsTTY:  &tty,
		Prompt: func(string) (string, error) { return "yes", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestConfirmProduction_DryRunAndInternal(t *testing.T) {
	if err := ConfirmProduction(ConfirmOpts{Target: "play_store", Track: "production", DryRun: true}); err != nil {
		t.Fatal(err)
	}
	if err := ConfirmProduction(ConfirmOpts{Target: "play_store", Track: "internal"}); err != nil {
		t.Fatal(err)
	}
}
