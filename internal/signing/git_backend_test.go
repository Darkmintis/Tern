package signing_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/darkmintis/Tern/internal/signing"
)

func writeFakeGit(t *testing.T) (gitPath string) {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "git.log")
	gitPath = filepath.Join(dir, "git")
	// Minimal git stub: clone creates dest/.git; other mutating commands succeed.
	script := fmt.Sprintf(`#!/bin/sh
echo "$*" >> %q
while [ "$1" = "-C" ]; do shift 2; done
case "$1" in
  clone)
    dest="$5"
    mkdir -p "$dest/.git"
    exit 0
    ;;
  pull|add|push) exit 0 ;;
  commit) exit 0 ;;
  *) exit 1 ;;
esac
`, logPath)
	if err := os.WriteFile(gitPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return gitPath
}

func TestGitBackendPullCloneAndPull(t *testing.T) {
	git := writeFakeGit(t)
	dest := filepath.Join(t.TempDir(), "certs")
	b := signing.GitBackend{RepoURL: "git@example.com:org/certs.git", GitPath: git}

	if err := b.Pull(context.Background(), dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, ".git")); err != nil {
		t.Fatalf("clone should create .git: %v", err)
	}
	// Second pull uses existing checkout.
	if err := b.Pull(context.Background(), dest); err != nil {
		t.Fatal(err)
	}
}

func TestGitBackendPullRequiresRepo(t *testing.T) {
	t.Setenv("CERT_REPO", "")
	b := signing.GitBackend{GitPath: writeFakeGit(t)}
	err := b.Pull(context.Background(), t.TempDir())
	if err == nil {
		t.Fatal("expected empty repo error")
	}
}

func TestGitBackendPushRequiresCheckout(t *testing.T) {
	b := signing.GitBackend{GitPath: writeFakeGit(t)}
	err := b.Push(context.Background(), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "not a git checkout") {
		t.Fatalf("got %v", err)
	}
}

func TestGitBackendPushOK(t *testing.T) {
	src := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	b := signing.GitBackend{GitPath: writeFakeGit(t)}
	if err := b.Push(context.Background(), src); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultCertSyncBackend(t *testing.T) {
	t.Setenv("CERT_REPO", "")
	if signing.DefaultCertSyncBackend() != nil {
		t.Fatal("expected nil without CERT_REPO")
	}
	t.Setenv("CERT_REPO", "git@example.com:org/certs.git")
	b := signing.DefaultCertSyncBackend()
	if _, ok := b.(signing.GitBackend); !ok {
		t.Fatalf("got %T", b)
	}
}

func TestDescribeBackend(t *testing.T) {
	if got := signing.DescribeBackend(nil); !strings.Contains(got, "none") {
		t.Fatalf("%q", got)
	}
	if got := signing.DescribeBackend(signing.GitBackend{}); !strings.Contains(got, "git") {
		t.Fatalf("%q", got)
	}
}

func TestCertSyncUsesGitBackendRepoURL(t *testing.T) {
	git := writeFakeGit(t)
	dest := filepath.Join(t.TempDir(), "certs")
	t.Setenv("CERT_REPO", "git@example.com:from-env.git")
	c := &signing.CertSync{Backend: signing.GitBackend{GitPath: git}}
	msg, err := c.Sync(context.Background(), signing.SyncOptions{Action: "pull", LocalDir: dest})
	if err != nil {
		t.Fatal(err)
	}
	if msg != "certs pulled" {
		t.Fatalf("%q", msg)
	}
	if _, err := os.Stat(filepath.Join(dest, ".git")); err != nil {
		t.Fatal(err)
	}
}
