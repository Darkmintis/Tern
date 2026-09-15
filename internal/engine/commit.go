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

var commitVersionRe = regexp.MustCompile(`(?m)^version:\s*(.+)`)

func runGitCommit(root, message string, dryRun bool) (string, error) {
	// Resolve $version placeholder in commit message.
	ver := "0.0.0"
	pub := filepath.Join(root, "pubspec.yaml")
	data, err := os.ReadFile(pub)
	if err == nil {
		if m := commitVersionRe.FindSubmatch(data); m != nil {
			ver = strings.TrimSpace(string(m[1]))
		}
	}
	tag := bump.TagName("", ver)
	msg := strings.ReplaceAll(message, "$version", tag)
	if msg == "" {
		msg = "release: v" + tag
	}

	if dryRun {
		return "dry-run: would git commit -m \"" + msg + "\"", nil
	}

	// Stage pubspec.yaml (the file bump modifies).
	cmd := exec.Command("git", "add", "pubspec.yaml")
	cmd.Dir = root
	if err := cmd.Run(); err != nil {
		return "", ternerrors.Wrap(ternerrors.ClassExec, "git add pubspec.yaml", err)
	}

	// Commit.
	cmd = exec.Command("git", "commit", "-m", msg)
	cmd.Dir = root
	if err := cmd.Run(); err != nil {
		return "", ternerrors.Wrap(ternerrors.ClassExec, "git commit", err)
	}

	return "committed: " + msg, nil
}
