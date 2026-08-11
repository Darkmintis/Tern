package config

import (
	"fmt"
	"strings"
	"unicode"

	ternerrors "github.com/darkmintis/Tern/internal/errors"
)

// tokenizeStep splits a Ternfile step line, honoring "double" and 'single' quotes.
func tokenizeStep(line string) ([]string, error) {
	var parts []string
	var b strings.Builder
	inQuote := rune(0)
	escape := false

	flush := func() {
		if b.Len() == 0 {
			return
		}
		parts = append(parts, b.String())
		b.Reset()
	}

	for _, r := range line {
		if escape {
			b.WriteRune(r)
			escape = false
			continue
		}
		if r == '\\' && inQuote != 0 {
			escape = true
			continue
		}
		if inQuote != 0 {
			if r == inQuote {
				inQuote = 0
				continue
			}
			b.WriteRune(r)
			continue
		}
		if r == '"' || r == '\'' {
			inQuote = r
			continue
		}
		if unicode.IsSpace(r) {
			flush()
			continue
		}
		b.WriteRune(r)
	}
	if inQuote != 0 {
		return nil, ternerrors.New(ternerrors.ClassConfig, "unclosed quote in step")
	}
	if escape {
		return nil, ternerrors.New(ternerrors.ClassConfig, "trailing escape in step")
	}
	flush()
	return parts, nil
}

func kvPrefix(token, key string) (string, bool) {
	pref := key + ":"
	if !strings.HasPrefix(token, pref) {
		return "", false
	}
	return strings.TrimPrefix(token, pref), true
}

func applyReleaseExtras(s *Step, extras []string) error {
	for _, extra := range extras {
		if v, ok := kvPrefix(extra, "track"); ok {
			s.Track = v
			continue
		}
		if v, ok := kvPrefix(extra, "release_name"); ok {
			strategy, custom, err := parseReleaseNameValue(v)
			if err != nil {
				return err
			}
			s.ReleaseNameStrategy = strategy
			s.ReleaseNameCustom = custom
			continue
		}
		if v, ok := kvPrefix(extra, "notes"); ok {
			mode, text, file, err := parseNotesValue(v)
			if err != nil {
				return err
			}
			s.NotesMode = mode
			s.NotesText = text
			s.NotesFile = file
			continue
		}
		if v, ok := kvPrefix(extra, "notes_locale"); ok {
			s.NotesLocale = v
			continue
		}
		return ternerrors.New(ternerrors.ClassConfig, fmt.Sprintf("unknown option %q", extra))
	}
	return nil
}

func parseReleaseNameValue(v string) (strategy, custom string, err error) {
	v = strings.TrimSpace(v)
	switch v {
	case "version", "version_build", "version_code", "semver_plus", "name_version",
		"date", "version_date", "git_tag", "git_sha", "none":
		return v, "", nil
	case "custom":
		return "", "", ternerrors.New(ternerrors.ClassConfig, `use release_name:"Your title" for a custom name`)
	default:
		if v == "" {
			return "version", "", nil
		}
		return "custom", v, nil
	}
}

func parseNotesValue(v string) (mode, text, file string, err error) {
	v = strings.TrimSpace(v)
	switch {
	case v == "" || v == "default":
		return "default", "", "", nil
	case v == "none":
		return "none", "", "", nil
	case strings.HasPrefix(v, "file:"):
		f := strings.TrimPrefix(v, "file:")
		if f == "" {
			return "", "", "", ternerrors.New(ternerrors.ClassConfig, "notes:file: requires a path")
		}
		return "file", "", f, nil
	default:
		return "text", v, "", nil
	}
}
