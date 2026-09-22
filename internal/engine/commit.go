package engine

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/darkmintis/Tern/internal/bump"
	ternerrors "github.com/darkmintis/Tern/internal/errors"
)

var commitVersionRe = regexp.MustCompile(`(?m)^version:\s*(.+)`)

// CommitPubspec stages pubspec.yaml (or all files with --all) and commits.
// $version in the message is replaced with the current pubspec version.
func (e *Engine) CommitPubspec(ctx context.Context, root, message string, all, dryRun bool) (string, error) {
	return runGitCommit(root, message, all, dryRun)
}

// TagVersion creates a git tag from the pubspec version.
func (e *Engine) TagVersion(ctx context.Context, root, prefix string, dryRun bool) (string, error) {
	tag, msg, err := runGitTag(root, prefix, dryRun)
	if err == nil && tag != "" && e != nil {
		e.lastGitTag = tag
	}
	return msg, err
}

func runGitCommit(root, message string, all, dryRun bool) (string, error) {
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
		if all {
			return "dry-run: would git add -A && git commit -m \"" + msg + "\"", nil
		}
		return "dry-run: would git commit -m \"" + msg + "\"", nil
	}

	// Stage files.
	if all {
		cmd := exec.Command("git", "add", "-A")
		cmd.Dir = root
		if err := cmd.Run(); err != nil {
			return "", ternerrors.Wrap(ternerrors.ClassExec, "git add -A", err)
		}
	} else {
		cmd := exec.Command("git", "add", "pubspec.yaml")
		cmd.Dir = root
		if err := cmd.Run(); err != nil {
			return "", ternerrors.Wrap(ternerrors.ClassExec, "git add pubspec.yaml", err)
		}
	}

	// Commit.
	cmd := exec.Command("git", "commit", "-m", msg)
	cmd.Dir = root
	if err := cmd.Run(); err != nil {
		return "", ternerrors.Wrap(ternerrors.ClassExec, "git commit", err)
	}

	return "committed: " + msg, nil
}
