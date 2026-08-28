package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/darkmintis/Tern/internal/config"
)

const (
	DirName  = ".tern"
	FileName = "history.json"
)

// Record is a single release entry.
type Record struct {
	Version      string          `json:"version"`
	Build        int             `json:"build"`
	Platform     config.Platform `json:"platform"`
	Target       string          `json:"target"`
	Track        string          `json:"track"`
	ArtifactPath string          `json:"artifact_path"`
	ArtifactSHA  string          `json:"artifact_sha256,omitempty"`
	ReleasedAt   time.Time       `json:"released_at"`
	GitTag       string          `json:"git_tag,omitempty"`
	Lane         string          `json:"lane,omitempty"`
	Rollout      float64         `json:"rollout,omitempty"`
}

// History is the full release history.
type History struct {
	Releases []Record `json:"releases"`
}

func historyPath(projectRoot string) string {
	return filepath.Join(projectRoot, DirName, FileName)
}

// Load reads the release history from disk.
func Load(projectRoot string) (History, error) {
	data, err := os.ReadFile(historyPath(projectRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return History{}, nil
		}
		return History{}, err
	}
	var h History
	if err := json.Unmarshal(data, &h); err != nil {
		return History{}, err
	}
	return h, nil
}

// Save writes the release history to disk.
func Save(projectRoot string, h History) error {
	dir := filepath.Join(projectRoot, DirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(historyPath(projectRoot), data, 0o644)
}

// Append adds a release record and saves.
func Append(projectRoot string, rec Record) error {
	h, err := Load(projectRoot)
	if err != nil {
		return err
	}
	h.Releases = append(h.Releases, rec)
	return Save(projectRoot, h)
}

// Last returns the most recent release, or nil if empty.
func Last(projectRoot string) (*Record, error) {
	h, err := Load(projectRoot)
	if err != nil {
		return nil, err
	}
	if len(h.Releases) == 0 {
		return nil, nil
	}
	return &h.Releases[len(h.Releases)-1], nil
}

// LastForTrack returns the most recent release for a given track.
func LastForTrack(projectRoot, track string) (*Record, error) {
	h, err := Load(projectRoot)
	if err != nil {
		return nil, err
	}
	for i := len(h.Releases) - 1; i >= 0; i-- {
		if h.Releases[i].Track == track {
			return &h.Releases[i], nil
		}
	}
	return nil, nil
}
