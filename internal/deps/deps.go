package deps

import (
	"os"
	"path/filepath"

	"github.com/darkmintis/Tern/internal/fingerprint"
)

const stateFile = ".tern/deps.fingerprint"

// ShouldSkipPubGet returns true when lockfile fingerprint matches last successful resolve.
func ShouldSkipPubGet(projectRoot string) (bool, string, error) {
	sum, err := fingerprint.LockfileHash(projectRoot)
	if err != nil {
		return false, "", err
	}
	prev, err := os.ReadFile(filepath.Join(projectRoot, stateFile))
	if err != nil {
		return false, sum, nil
	}
	return string(prev) == sum, sum, nil
}

// MarkResolved stores the current lockfile fingerprint after a successful pub get.
func MarkResolved(projectRoot, sum string) error {
	dir := filepath.Join(projectRoot, ".tern")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if sum == "" {
		var err error
		sum, err = fingerprint.LockfileHash(projectRoot)
		if err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(projectRoot, stateFile), []byte(sum), 0o644)
}
