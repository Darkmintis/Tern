package signing

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	ternerrors "github.com/darkmintis/Tern/internal/errors"
)

// GitBackend syncs an unencrypted cert directory via git clone/pull/push.
// Encryption (age) remains Phase 1.5; this unblocks teams that already keep
// signing material in a private git repo referenced by env:CERT_REPO.
type GitBackend struct {
	RepoURL string // set by CertSync.Sync from env
	GitPath string
}

func (g GitBackend) git() string {
	if g.GitPath != "" {
		return g.GitPath
	}
	return "git"
}

func (g GitBackend) repo() string {
	if strings.TrimSpace(g.RepoURL) != "" {
		return strings.TrimSpace(g.RepoURL)
	}
	return strings.TrimSpace(os.Getenv("CERT_REPO"))
}

func (g GitBackend) Pull(ctx context.Context, destDir string) error {
	repo := g.repo()
	if repo == "" {
		return ternerrors.New(ternerrors.ClassSign, "CERT_REPO / sync repo URL is empty")
	}
	if destDir == "" {
		destDir = "secrets/certs"
	}
	if st, err := os.Stat(filepath.Join(destDir, ".git")); err == nil && st.IsDir() {
		cmd := exec.CommandContext(ctx, g.git(), "-C", destDir, "pull", "--ff-only")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return ternerrors.WrapHint(ternerrors.ClassSign, "git pull certs", string(out), err)
		}
		return nil
	}
	_ = os.MkdirAll(filepath.Dir(destDir), 0o755)
	cmd := exec.CommandContext(ctx, g.git(), "clone", "--depth", "1", repo, destDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ternerrors.WrapHint(ternerrors.ClassSign, "git clone certs", string(out), err)
	}
	return nil
}

func (g GitBackend) Push(ctx context.Context, srcDir string) error {
	if srcDir == "" {
		srcDir = "secrets/certs"
	}
	if _, err := os.Stat(filepath.Join(srcDir, ".git")); err != nil {
		return ternerrors.NewHint(ternerrors.ClassSign,
			"cert sync push: "+srcDir+" is not a git checkout",
			"run sync_certs pull first, or clone CERT_REPO into that directory")
	}
	add := exec.CommandContext(ctx, g.git(), "-C", srcDir, "add", "-A")
	if out, err := add.CombinedOutput(); err != nil {
		return ternerrors.WrapHint(ternerrors.ClassSign, "git add certs", string(out), err)
	}
	commit := exec.CommandContext(ctx, g.git(), "-C", srcDir, "commit", "-m", "tern: sync certs")
	if out, err := commit.CombinedOutput(); err != nil {
		// Nothing to commit is OK.
		if !strings.Contains(string(out), "nothing to commit") {
			return ternerrors.WrapHint(ternerrors.ClassSign, "git commit certs", string(out), err)
		}
	}
	push := exec.CommandContext(ctx, g.git(), "-C", srcDir, "push")
	if out, err := push.CombinedOutput(); err != nil {
		return ternerrors.WrapHint(ternerrors.ClassSign, "git push certs", string(out), err)
	}
	return nil
}

// DefaultCertSyncBackend returns GitBackend when CERT_REPO is set, else nil
// (sync_certs stays reserved until a repo is configured).
func DefaultCertSyncBackend() SyncBackend {
	if strings.TrimSpace(os.Getenv("CERT_REPO")) == "" {
		return nil
	}
	return GitBackend{}
}

// DescribeBackend is used in doctor/dry-run messages.
func DescribeBackend(b SyncBackend) string {
	if b == nil {
		return "none (set CERT_REPO to enable git cert sync)"
	}
	if _, ok := b.(GitBackend); ok {
		return "git (CERT_REPO)"
	}
	return fmt.Sprintf("%T", b)
}
