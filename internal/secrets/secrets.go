package secrets

import (
	"os"
	"strings"

	ternerrors "github.com/darkmintis/Tern/internal/errors"
)

var weakPatterns = []string{
	"", "changeme", "default", "secret", "password", "123456", "password123",
}

// CheckEnvPresent ensures name is set and non-empty.
func CheckEnvPresent(name string) error {
	v := os.Getenv(name)
	if strings.TrimSpace(v) == "" {
		return ternerrors.New(ternerrors.ClassDoctor, "missing required secret env:"+name)
	}
	return nil
}

// CheckEnvStrong ensures the value is not a known weak pattern.
func CheckEnvStrong(name string) error {
	if err := CheckEnvPresent(name); err != nil {
		return err
	}
	v := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	for _, w := range weakPatterns {
		if v == w {
			return ternerrors.New(ternerrors.ClassDoctor, "weak/default secret for env:"+name)
		}
	}
	return nil
}

// IsWeak reports whether value matches a weak pattern.
func IsWeak(value string) bool {
	v := strings.ToLower(strings.TrimSpace(value))
	for _, w := range weakPatterns {
		if v == w {
			return true
		}
	}
	return false
}

// FileReadable checks path exists and is readable.
func FileReadable(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return ternerrors.Wrap(ternerrors.ClassDoctor, "secret file not readable: "+path, err)
	}
	_ = f.Close()
	return nil
}

// ResolveEnv returns the value of env name or DoctorError.
func ResolveEnv(name string) (string, error) {
	if err := CheckEnvPresent(name); err != nil {
		return "", err
	}
	return os.Getenv(name), nil
}

// Redact masks values that look like secrets.
func Redact(s string) string {
	if looksSecret(s) {
		return "***"
	}
	return s
}

func looksSecret(s string) bool {
	lower := strings.ToLower(s)
	if strings.Contains(lower, "key") || strings.Contains(lower, "token") ||
		strings.Contains(lower, "secret") || strings.Contains(lower, "password") {
		return true
	}
	return len(s) >= 16
}
