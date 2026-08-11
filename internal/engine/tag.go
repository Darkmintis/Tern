package engine

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/darkmintis/Tern/internal/bump"
	"github.com/darkmintis/Tern/internal/config"
	ternerrors "github.com/darkmintis/Tern/internal/errors"
)

func runGitTag(root, prefix string, dryRun bool) (string, error) {
	ver := "0.0.0"
	pub := filepath.Join(root, "pubspec.yaml")
	if data, err := os.ReadFile(pub); err == nil {
		if r, berr := bump.BumpVersion(root, config.BumpPatch, true); berr == nil && r.Old != "" {
			_ = data
			ver = r.Old
			ver = trimVersionLine(ver)
		}
	}
	tag := bump.TagName(prefix, ver)
	if dryRun {
		return "dry-run: would git tag " + tag, nil
	}
	cmd := exec.Command("git", "tag", tag)
	cmd.Dir = root
	if err := cmd.Run(); err != nil {
		return "", ternerrors.Wrap(ternerrors.ClassExec, "git tag", err)
	}
	return "created tag " + tag, nil
}

func trimVersionLine(s string) string {
	s = filepath.Base(s)
	const p = "version:"
	if len(s) > len(p) && s[:len(p)] == p {
		s = s[len(p):]
	}
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	return s
}
