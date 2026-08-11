package config

import (
	"bufio"
	"fmt"
	"strings"

	ternerrors "github.com/darkmintis/Tern/internal/errors"
)

// ParseDSL parses Ternfile DSL text.
func ParseDSL(src string) (*Config, error) {
	cfg := &Config{Lanes: map[string]Lane{}}
	sc := bufio.NewScanner(strings.NewReader(src))
	lineNum := 0
	var current *Lane

	for sc.Scan() {
		lineNum++
		raw := sc.Text()
		line := strings.TrimRight(raw, " \t")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if strings.HasPrefix(trimmed, "lane ") && strings.HasSuffix(trimmed, ":") {
			name := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "lane "), ":"))
			if name == "" {
				return nil, ternerrors.New(ternerrors.ClassConfig, fmt.Sprintf("line %d: empty lane name", lineNum))
			}
			l := Lane{Name: name, Steps: nil}
			cfg.Lanes[name] = l
			current = &Lane{Name: name}
			continue
		}

		if current == nil {
			return nil, ternerrors.New(ternerrors.ClassConfig, fmt.Sprintf("line %d: step outside lane", lineNum))
		}
		if !strings.HasPrefix(raw, "  ") && !strings.HasPrefix(raw, "\t") {
			return nil, ternerrors.New(ternerrors.ClassConfig, fmt.Sprintf("line %d: steps must be indented", lineNum))
		}

		step, err := parseStep(trimmed)
		if err != nil {
			return nil, ternerrors.Wrap(ternerrors.ClassConfig, fmt.Sprintf("line %d", lineNum), err)
		}
		step.Raw = trimmed
		current.Steps = append(current.Steps, step)
		cfg.Lanes[current.Name] = *current
	}
	if err := sc.Err(); err != nil {
		return nil, ternerrors.Wrap(ternerrors.ClassConfig, "scanning Ternfile", err)
	}
	return cfg, nil
}

func parseStep(line string) (Step, error) {
	parts, err := tokenizeStep(line)
	if err != nil {
		return Step{}, err
	}
	if len(parts) == 0 {
		return Step{}, ternerrors.New(ternerrors.ClassConfig, "empty step")
	}
	switch parts[0] {
	case "build":
		return parseBuild(parts)
	case "sign":
		return parseSign(parts)
	case "upload":
		return parseUpload(parts)
	case "ship":
		return parseShip(parts)
	case "bump":
		return parseBump(parts)
	case "tag":
		return parseTag(parts)
	case "sync_certs":
		return parseSyncCerts(parts)
	case "notify":
		return parseNotify(parts)
	default:
		return Step{}, ternerrors.New(ternerrors.ClassConfig, fmt.Sprintf("unknown step kind %q", parts[0]))
	}
}

func parseBuild(parts []string) (Step, error) {
	// build android release [aab|apk] [flavor:name] [scheme:name]
	if len(parts) < 3 {
		return Step{}, ternerrors.New(ternerrors.ClassConfig, "build requires platform and mode")
	}
	p, err := parsePlatform(parts[1])
	if err != nil {
		return Step{}, err
	}
	m, err := parseMode(parts[2])
	if err != nil {
		return Step{}, err
	}
	s := Step{Kind: StepBuild, Platform: p, Mode: m}
	for _, extra := range parts[3:] {
		switch extra {
		case "aab":
			s.ArtifactKind = ArtifactAAB
		case "apk":
			s.ArtifactKind = ArtifactAPK
		default:
			if v, ok := kvPrefix(extra, "flavor"); ok {
				if v == "" {
					return Step{}, ternerrors.New(ternerrors.ClassConfig, "flavor: requires a name")
				}
				s.Flavor = v
				continue
			}
			if v, ok := kvPrefix(extra, "scheme"); ok {
				if v == "" {
					return Step{}, ternerrors.New(ternerrors.ClassConfig, "scheme: requires a name")
				}
				s.Scheme = v
				continue
			}
			return Step{}, ternerrors.New(ternerrors.ClassConfig, fmt.Sprintf("unknown build option %q", extra))
		}
	}
	if s.ArtifactKind == "" && p == PlatformAndroid && m == ModeRelease {
		s.ArtifactKind = ArtifactAAB
	}
	if s.ArtifactKind == "" && p == PlatformAndroid && m == ModeDebug {
		s.ArtifactKind = ArtifactAPK
	}
	if s.ArtifactKind == "" && p == PlatformIOS {
		s.ArtifactKind = ArtifactIPA
	}
	return s, nil
}

func parseShip(parts []string) (Step, error) {
	// ship android from last to play_store track:internal
	// ship ios from path/to.ipa to testflight
	if len(parts) < 6 {
		return Step{}, ternerrors.New(ternerrors.ClassConfig,
			"ship requires: ship <platform> from <last|path> to <target>")
	}
	p, err := parsePlatform(parts[1])
	if err != nil {
		return Step{}, err
	}
	if parts[2] != "from" {
		return Step{}, ternerrors.New(ternerrors.ClassConfig, "ship: expected 'from'")
	}
	from := parts[3]
	if parts[4] != "to" {
		return Step{}, ternerrors.New(ternerrors.ClassConfig, "ship: expected 'to'")
	}
	target := parts[5]
	switch target {
	case "play_store", "testflight", "app_store":
	default:
		return Step{}, ternerrors.New(ternerrors.ClassConfig, fmt.Sprintf("unknown ship target %q", target))
	}
	s := Step{Kind: StepShip, Platform: p, ShipFrom: from, UploadTarget: target}
	if err := applyReleaseExtras(&s, parts[6:]); err != nil {
		return Step{}, err
	}
	return s, nil
}

func parseSign(parts []string) (Step, error) {
	// sign android with keystore env:ANDROID_KEYSTORE
	if len(parts) < 5 {
		return Step{}, ternerrors.New(ternerrors.ClassConfig, "sign requires: sign <platform> with <keystore|cert> env:NAME")
	}
	p, err := parsePlatform(parts[1])
	if err != nil {
		return Step{}, err
	}
	if parts[2] != "with" {
		return Step{}, ternerrors.New(ternerrors.ClassConfig, "sign: expected 'with'")
	}
	with := parts[3]
	if with != "keystore" && with != "cert" {
		return Step{}, ternerrors.New(ternerrors.ClassConfig, "sign: expected keystore or cert")
	}
	envRef, err := parseEnvRef(parts[4])
	if err != nil {
		return Step{}, err
	}
	return Step{Kind: StepSign, Platform: p, SignWith: with, EnvRef: envRef}, nil
}

func parseUpload(parts []string) (Step, error) {
	// upload android to play_store track:internal
	if len(parts) < 4 {
		return Step{}, ternerrors.New(ternerrors.ClassConfig, "upload requires: upload <platform> to <target>")
	}
	p, err := parsePlatform(parts[1])
	if err != nil {
		return Step{}, err
	}
	if parts[2] != "to" {
		return Step{}, ternerrors.New(ternerrors.ClassConfig, "upload: expected 'to'")
	}
	target := parts[3]
	switch target {
	case "play_store", "testflight", "app_store":
	default:
		return Step{}, ternerrors.New(ternerrors.ClassConfig, fmt.Sprintf("unknown upload target %q", target))
	}
	s := Step{Kind: StepUpload, Platform: p, UploadTarget: target}
	if err := applyReleaseExtras(&s, parts[4:]); err != nil {
		return Step{}, err
	}
	return s, nil
}

func parseBump(parts []string) (Step, error) {
	// bump version patch
	level := BumpPatch
	if len(parts) >= 3 {
		switch parts[2] {
		case "major", "minor", "patch", "build":
			level = BumpLevel(parts[2])
		default:
			return Step{}, ternerrors.New(ternerrors.ClassConfig, fmt.Sprintf("unknown bump level %q", parts[2]))
		}
	}
	return Step{Kind: StepBump, BumpLevel: level}, nil
}

func parseTag(parts []string) (Step, error) {
	// tag git prefix:v
	prefix := "v"
	for _, extra := range parts[1:] {
		if strings.HasPrefix(extra, "prefix:") {
			prefix = strings.TrimPrefix(extra, "prefix:")
		}
	}
	return Step{Kind: StepTag, TagPrefix: prefix}, nil
}

func parseSyncCerts(parts []string) (Step, error) {
	// sync_certs pull repo:env:CERT_REPO
	if len(parts) < 2 {
		return Step{}, ternerrors.New(ternerrors.ClassConfig, "sync_certs requires pull or push")
	}
	action := parts[1]
	if action != "pull" && action != "push" {
		return Step{}, ternerrors.New(ternerrors.ClassConfig, "sync_certs: expected pull or push")
	}
	s := Step{Kind: StepSyncCerts, SyncAction: action}
	for _, extra := range parts[2:] {
		if strings.HasPrefix(extra, "repo:env:") {
			s.EnvRef = strings.TrimPrefix(extra, "repo:env:")
		} else if strings.HasPrefix(extra, "env:") {
			s.EnvRef = strings.TrimPrefix(extra, "env:")
		}
	}
	return s, nil
}

func parseNotify(parts []string) (Step, error) {
	// notify slack env:SLACK_WEBHOOK
	if len(parts) < 3 {
		return Step{}, ternerrors.New(ternerrors.ClassConfig, "notify requires: notify <slack|discord> env:NAME")
	}
	via := parts[1]
	if via != "slack" && via != "discord" {
		return Step{}, ternerrors.New(ternerrors.ClassConfig, "notify: expected slack or discord")
	}
	envRef, err := parseEnvRef(parts[2])
	if err != nil {
		return Step{}, err
	}
	return Step{Kind: StepNotify, NotifyVia: via, EnvRef: envRef}, nil
}

func parsePlatform(s string) (Platform, error) {
	switch s {
	case "android":
		return PlatformAndroid, nil
	case "ios":
		return PlatformIOS, nil
	default:
		return "", ternerrors.New(ternerrors.ClassConfig, fmt.Sprintf("unknown platform %q", s))
	}
}

func parseMode(s string) (Mode, error) {
	switch s {
	case "debug":
		return ModeDebug, nil
	case "release":
		return ModeRelease, nil
	default:
		return "", ternerrors.New(ternerrors.ClassConfig, fmt.Sprintf("unknown mode %q", s))
	}
}

func parseEnvRef(s string) (string, error) {
	if !strings.HasPrefix(s, "env:") {
		return "", ternerrors.New(ternerrors.ClassConfig, "expected env:NAME reference")
	}
	name := strings.TrimPrefix(s, "env:")
	if name == "" {
		return "", ternerrors.New(ternerrors.ClassConfig, "empty env reference")
	}
	return name, nil
}
