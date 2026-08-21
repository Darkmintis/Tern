package projectmeta

import (
	"strconv"
	"strings"

	ternerrors "github.com/darkmintis/Tern/internal/errors"
)

// LocalVersion is the parsed Flutter pubspec version.
type LocalVersion struct {
	Raw       string // 1.2.3+45
	Marketing string // 1.2.3
	Build     string // 45 (may be empty)
	Code      int64  // numeric build / versionCode; 0 if missing
}

// FlutterLocalVersion reads and parses pubspec version for store comparisons.
func FlutterLocalVersion(projectRoot string) (LocalVersion, error) {
	raw, err := FlutterVersion(projectRoot)
	if err != nil {
		return LocalVersion{}, err
	}
	marketing, build := splitVersion(raw)
	out := LocalVersion{Raw: raw, Marketing: marketing, Build: build}
	if build == "" {
		return out, nil
	}
	n, err := strconv.ParseInt(strings.TrimSpace(build), 10, 64)
	if err != nil || n < 0 {
		return out, ternerrors.New(ternerrors.ClassConfig,
			"pubspec build number must be a non-negative integer (got "+build+")")
	}
	out.Code = n
	return out, nil
}

func splitVersion(ver string) (marketing, build string) {
	ver = strings.TrimSpace(ver)
	if ver == "" {
		return "", ""
	}
	if i := strings.IndexByte(ver, '+'); i >= 0 {
		return ver[:i], ver[i+1:]
	}
	return ver, ""
}
