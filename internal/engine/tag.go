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

// runGitTag creates a git tag from the pubspec version. If the tag already
// exists, it skips with a clear message (re-running release after success).
func runGitTag(root, prefix string, dryRun bool) (tagName, message string, err error) {
	ver := "0.0.0"
	pub := filepath.Join(root, "pubspec.yaml")
	data, rerr := os.ReadFile(pub)
	if rerr == nil {
		if m := tagVersionRe.FindSubmatch(data); m != nil {
			ver = strings.TrimSpace(string(m[1]))
		}
	}
	tag := bump.TagName(prefix, ver)
	if dryRun {
		return tag, "dry-run: would git tag " + tag, nil
	}
	check := exec.Command("git", "rev-parse", "--verify", "refs/tags/"+tag)
	check.Dir = root
	if check.Run() == nil {
		return tag, "tag " + tag + " already exists (skipped)", nil
	}
	cmd := exec.Command("git", "tag", tag)
	cmd.Dir = root
	if err := cmd.Run(); err != nil {
		return "", "", ternerrors.WrapHint(ternerrors.ClassExec, "git tag "+tag+" failed",
			"if the tag exists for a different commit, delete it locally (`git tag -d "+tag+"`) or bump version; upload may already have succeeded — use tern ship last to retry upload only",
			err)
	}
	return tag, "created tag " + tag, nil
}
