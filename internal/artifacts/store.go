package artifacts

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/darkmintis/Tern/internal/config"
	ternerrors "github.com/darkmintis/Tern/internal/errors"
)

const DirName = ".tern/artifacts"

// Record is metadata for a built artifact.
type Record struct {
	Platform    config.Platform `json:"platform"`
	Kind        string          `json:"kind"`
	Path        string          `json:"path"`
	SHA256      string          `json:"sha256"`
	Version     string          `json:"version,omitempty"`
	Fingerprint string          `json:"fingerprint,omitempty"`
	BuiltAt     time.Time       `json:"built_at"`
	SizeBytes   int64           `json:"size_bytes"`
}

// Dir returns .tern/artifacts under projectRoot.
func Dir(projectRoot string) string {
	return filepath.Join(projectRoot, DirName)
}

func metaPath(projectRoot string, platform config.Platform) string {
	return filepath.Join(Dir(projectRoot), string(platform)+".json")
}

// EnsureDir creates the artifacts directory.
func EnsureDir(projectRoot string) error {
	return os.MkdirAll(Dir(projectRoot), 0o755)
}

// HashFile returns sha256 of path.
func HashFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// Save writes metadata for a platform after a successful build.
func Save(projectRoot string, rec Record) error {
	if err := EnsureDir(projectRoot); err != nil {
		return ternerrors.Wrap(ternerrors.ClassBuild, "artifacts dir", err)
	}
	if rec.BuiltAt.IsZero() {
		rec.BuiltAt = time.Now().UTC()
	}
	if rec.SHA256 == "" && rec.Path != "" {
		if _, err := os.Stat(rec.Path); err == nil {
			sum, size, herr := HashFile(rec.Path)
			if herr != nil {
				return ternerrors.Wrap(ternerrors.ClassBuild, "hash artifact", herr)
			}
			rec.SHA256 = sum
			rec.SizeBytes = size
		}
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(metaPath(projectRoot, rec.Platform), data, 0o644)
}

// Load reads last artifact metadata for platform.
func Load(projectRoot string, platform config.Platform) (Record, error) {
	data, err := os.ReadFile(metaPath(projectRoot, platform))
	if err != nil {
		return Record{}, ternerrors.Wrap(ternerrors.ClassUpload, "no saved artifact for "+string(platform), err)
	}
	var rec Record
	if err := json.Unmarshal(data, &rec); err != nil {
		return Record{}, ternerrors.Wrap(ternerrors.ClassUpload, "parse artifact metadata", err)
	}
	return rec, nil
}

// ResolvePath returns an explicit path or the last saved artifact for platform.
func ResolvePath(projectRoot string, platform config.Platform, explicit string) (string, Record, error) {
	if explicit != "" && explicit != "last" {
		sum, size, err := HashFile(explicit)
		if err != nil {
			return "", Record{}, ternerrors.Wrap(ternerrors.ClassUpload, "artifact", err)
		}
		return explicit, Record{Platform: platform, Path: explicit, SHA256: sum, SizeBytes: size}, nil
	}
	rec, err := Load(projectRoot, platform)
	if err != nil {
		return "", Record{}, err
	}
	if _, err := os.Stat(rec.Path); err != nil {
		return "", Record{}, ternerrors.Wrap(ternerrors.ClassUpload, "saved artifact missing on disk", err)
	}
	return rec.Path, rec, nil
}

// Verify checks path exists, matches expected kind extension, and optional sha.
func Verify(path, kind, expectSHA string) error {
	info, err := os.Stat(path)
	if err != nil {
		return ternerrors.Wrap(ternerrors.ClassUpload, "artifact missing", err)
	}
	if info.IsDir() && kind != "ipa" {
		// ipa may be a directory before resolve; after resolve should be file
		return ternerrors.New(ternerrors.ClassUpload, "artifact is a directory: "+path)
	}
	if expectSHA != "" {
		sum, _, err := HashFile(path)
		if err != nil {
			return err
		}
		if sum != expectSHA {
			return ternerrors.NewHint(ternerrors.ClassUpload,
				fmt.Sprintf("artifact hash mismatch (got %s want %s)", sum[:12], expectSHA[:12]),
				"rebuild the artifact; do not ship a tampered or stale file")
		}
	}
	return nil
}
