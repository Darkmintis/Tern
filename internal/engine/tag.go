package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/darkmintis/Tern/internal/bump"
	ternerrors "github.com/darkmintis/Tern/internal/errors"
)

var tagVersionRe = regexp.MustCompile(`(?m)^version:\s*(.+)`)

func runGitTag(root, prefix string, dryRun bool) (string, error) {
	ver := "0.0.0"
	pub := filepath.Join(root, "pubspec.yaml")
	data, err := os.ReadFile(pub)
	if err == nil {
		if m := tagVersionRe.FindSubmatch(data); m != nil {
			ver = strings.TrimSpace(string(m[1]))
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
