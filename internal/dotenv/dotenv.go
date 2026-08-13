package dotenv

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// LoadFile reads KEY=VALUE lines from path into the process environment.
// Existing env vars are not overwritten. Missing file is a no-op.
// Lines starting with # and blank lines are ignored. Values may be "quoted" or 'quoted'.
func LoadFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue // shell / CI wins
		}
		val = strings.TrimSpace(val)
		val = unquote(val)
		_ = os.Setenv(key, val)
	}
	return sc.Err()
}

// LoadProject loads <dir>/.env if present.
func LoadProject(dir string) error {
	if dir == "" {
		dir, _ = os.Getwd()
	}
	return LoadFile(filepath.Join(dir, ".env"))
}

func unquote(v string) string {
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			return v[1 : len(v)-1]
		}
	}
	return v
}
