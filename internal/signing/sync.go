package signing

import (
	"context"
	"fmt"

	ternerrors "github.com/darkmintis/Tern/internal/errors"
	"github.com/darkmintis/Tern/internal/secrets"
)

// SyncBackend is the Phase 1.5 Match-killer abstraction (age-encrypted cert store).
// Full git/S3 implementation lands in Phase 1.5; hooks exist now so signing stays framework-agnostic.
type SyncBackend interface {
	Pull(ctx context.Context, destDir string) error
	Push(ctx context.Context, srcDir string) error
}

// SyncOptions configure cert sync.
type SyncOptions struct {
	Action   string // pull | push
	RepoEnv  string // env var holding repo URL / path
	LocalDir string
	DryRun   bool
}

// CertSync orchestrates encrypted cert repository sync.
type CertSync struct {
	Backend SyncBackend
}

// Sync runs pull or push (or dry-run description).
func (c *CertSync) Sync(ctx context.Context, opts SyncOptions) (string, error) {
	if opts.RepoEnv != "" {
		if _, err := secrets.ResolveEnv(opts.RepoEnv); err != nil {
			return "", ternerrors.Wrap(ternerrors.ClassSign, "cert sync repo", err)
		}
	}
	if opts.DryRun || c.Backend == nil {
		return fmt.Sprintf("dry-run: would sync_certs %s (repo env:%s)", opts.Action, opts.RepoEnv), nil
	}
	switch opts.Action {
	case "pull":
		if err := c.Backend.Pull(ctx, opts.LocalDir); err != nil {
			return "", ternerrors.Wrap(ternerrors.ClassSign, "sync_certs pull", err)
		}
		return "certs pulled", nil
	case "push":
		if err := c.Backend.Push(ctx, opts.LocalDir); err != nil {
			return "", ternerrors.Wrap(ternerrors.ClassSign, "sync_certs push", err)
		}
		return "certs pushed", nil
	default:
		return "", ternerrors.New(ternerrors.ClassSign, "sync_certs: action must be pull or push")
	}
}

// NoopBackend is a placeholder SyncBackend.
type NoopBackend struct{}

func (NoopBackend) Pull(context.Context, string) error { return nil }
func (NoopBackend) Push(context.Context, string) error { return nil }
