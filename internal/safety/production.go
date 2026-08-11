package safety

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	ternerrors "github.com/darkmintis/Tern/internal/errors"
)

// IsProduction reports whether an upload target/track is a production gate.
// Play: track production (or prod). iOS: app_store (not testflight).
func IsProduction(target, track string) bool {
	t := strings.ToLower(strings.TrimSpace(target))
	tr := strings.ToLower(strings.TrimSpace(track))
	if t == "app_store" {
		return true
	}
	if t == "play_store" || t == "" {
		return tr == "production" || tr == "prod"
	}
	return false
}

// ConfirmOpts controls the production gate.
type ConfirmOpts struct {
	Target string
	Track  string
	// Yes skips interactive confirm (CLI --yes).
	Yes bool
	// DryRun skips the gate.
	DryRun bool
	// Prompt reads a line from the operator; defaults to stdin.
	Prompt func(question string) (string, error)
	// IsCI overrides CI detection (tests).
	IsCI *bool
	// IsTTY overrides TTY detection (tests).
	IsTTY *bool
}

// ConfirmProduction refuses or prompts before production uploads.
// In CI / non-TTY without --yes, returns an error. Interactive TTY prompts y/N.
func ConfirmProduction(opts ConfirmOpts) error {
	if opts.DryRun || !IsProduction(opts.Target, opts.Track) {
		return nil
	}
	if opts.Yes {
		return nil
	}

	label := describe(opts.Target, opts.Track)
	hint := "re-run with --yes to confirm a production upload (required in CI)"

	ci := inCI(opts.IsCI)
	tty := isInteractive(opts.IsTTY)
	if ci || !tty {
		return ternerrors.NewHint(ternerrors.ClassUpload,
			fmt.Sprintf("refusing production upload to %s without --yes", label),
			hint)
	}

	prompt := opts.Prompt
	if prompt == nil {
		prompt = defaultPrompt
	}
	ans, err := prompt(fmt.Sprintf("Upload to %s? Type 'yes' to continue: ", label))
	if err != nil {
		return ternerrors.WrapHint(ternerrors.ClassUpload, "production confirm failed", hint, err)
	}
	ans = strings.ToLower(strings.TrimSpace(ans))
	if ans != "yes" && ans != "y" {
		return ternerrors.NewHint(ternerrors.ClassUpload,
			"production upload cancelled",
			"re-run and type yes, or pass --yes in CI")
	}
	return nil
}

func describe(target, track string) string {
	t := strings.ToLower(strings.TrimSpace(target))
	tr := strings.TrimSpace(track)
	if t == "app_store" {
		return "App Store (production)"
	}
	if tr == "" {
		tr = "production"
	}
	return fmt.Sprintf("Play track %q", tr)
}

func inCI(override *bool) bool {
	if override != nil {
		return *override
	}
	for _, k := range []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "CIRCLECI", "BUILDKITE", "TF_BUILD"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" && v != "0" && !strings.EqualFold(v, "false") {
			return true
		}
	}
	return false
}

func isInteractive(override *bool) bool {
	if override != nil {
		return *override
	}
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func defaultPrompt(question string) (string, error) {
	fmt.Fprint(os.Stderr, question)
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil && len(strings.TrimSpace(line)) == 0 {
		return "", err
	}
	return line, nil
}
