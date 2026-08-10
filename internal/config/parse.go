package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	ternerrors "github.com/darkmintis/Tern/internal/errors"
	"gopkg.in/yaml.v3"
)

// Load reads Ternfile or ternfile.yaml from dir (prefer Ternfile).
func Load(dir string) (*Config, error) {
	dslPath := dir + "/Ternfile"
	yamlPath := dir + "/ternfile.yaml"
	if _, err := os.Stat(dslPath); err == nil {
		return ParseDSLFile(dslPath)
	}
	if _, err := os.Stat(yamlPath); err == nil {
		return ParseYAMLFile(yamlPath)
	}
	return nil, ternerrors.New(ternerrors.ClassConfig, "no Ternfile or ternfile.yaml found")
}

// ParseDSLFile parses the Ternfile DSL.
func ParseDSLFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, ternerrors.Wrap(ternerrors.ClassConfig, "reading Ternfile", err)
	}
	cfg, err := ParseDSL(string(data))
	if err != nil {
		return nil, err
	}
	cfg.Source = path
	return cfg, nil
}

// ParseYAMLFile parses ternfile.yaml into the same IR.
func ParseYAMLFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, ternerrors.Wrap(ternerrors.ClassConfig, "reading ternfile.yaml", err)
	}
	cfg, err := ParseYAML(data)
	if err != nil {
		return nil, err
	}
	cfg.Source = path
	return cfg, nil
}

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
	parts := strings.Fields(line)
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
	// build android release
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
	return Step{Kind: StepBuild, Platform: p, Mode: m}, nil
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
	for _, extra := range parts[4:] {
		if strings.HasPrefix(extra, "track:") {
			s.Track = strings.TrimPrefix(extra, "track:")
		}
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

// yamlFile is the structured escape hatch.
type yamlFile struct {
	Lanes map[string][]yamlStep `yaml:"lanes"`
}

type yamlStep struct {
	Build     *yamlBuild     `yaml:"build"`
	Sign      *yamlSign      `yaml:"sign"`
	Upload    *yamlUpload    `yaml:"upload"`
	Bump      *yamlBump      `yaml:"bump"`
	Tag       *yamlTag       `yaml:"tag"`
	SyncCerts *yamlSyncCerts `yaml:"sync_certs"`
	Notify    *yamlNotify    `yaml:"notify"`
}

type yamlBuild struct {
	Platform string `yaml:"platform"`
	Mode     string `yaml:"mode"`
}

type yamlSign struct {
	Platform string `yaml:"platform"`
	With     string `yaml:"with"`
	Env      string `yaml:"env"`
}

type yamlUpload struct {
	Platform string `yaml:"platform"`
	To       string `yaml:"to"`
	Track    string `yaml:"track"`
}

type yamlBump struct {
	Level string `yaml:"level"`
}

type yamlTag struct {
	Prefix string `yaml:"prefix"`
}

type yamlSyncCerts struct {
	Action string `yaml:"action"`
	Env    string `yaml:"env"`
}

type yamlNotify struct {
	Via string `yaml:"via"`
	Env string `yaml:"env"`
}

// ParseYAML parses ternfile.yaml into IR.
func ParseYAML(data []byte) (*Config, error) {
	var yf yamlFile
	if err := yaml.Unmarshal(data, &yf); err != nil {
		return nil, ternerrors.Wrap(ternerrors.ClassConfig, "parsing ternfile.yaml", err)
	}
	cfg := &Config{Lanes: map[string]Lane{}}
	for name, steps := range yf.Lanes {
		lane := Lane{Name: name}
		for i, ys := range steps {
			st, err := yamlToStep(ys)
			if err != nil {
				return nil, ternerrors.Wrap(ternerrors.ClassConfig, fmt.Sprintf("lane %s step %d", name, i), err)
			}
			lane.Steps = append(lane.Steps, st)
		}
		cfg.Lanes[name] = lane
	}
	return cfg, nil
}

func yamlToStep(ys yamlStep) (Step, error) {
	n := 0
	var s Step
	if ys.Build != nil {
		n++
		p, err := parsePlatform(ys.Build.Platform)
		if err != nil {
			return Step{}, err
		}
		m, err := parseMode(ys.Build.Mode)
		if err != nil {
			return Step{}, err
		}
		s = Step{Kind: StepBuild, Platform: p, Mode: m, Raw: "build"}
	}
	if ys.Sign != nil {
		n++
		p, err := parsePlatform(ys.Sign.Platform)
		if err != nil {
			return Step{}, err
		}
		s = Step{Kind: StepSign, Platform: p, SignWith: ys.Sign.With, EnvRef: ys.Sign.Env, Raw: "sign"}
	}
	if ys.Upload != nil {
		n++
		p, err := parsePlatform(ys.Upload.Platform)
		if err != nil {
			return Step{}, err
		}
		s = Step{Kind: StepUpload, Platform: p, UploadTarget: ys.Upload.To, Track: ys.Upload.Track, Raw: "upload"}
	}
	if ys.Bump != nil {
		n++
		s = Step{Kind: StepBump, BumpLevel: BumpLevel(ys.Bump.Level), Raw: "bump"}
		if s.BumpLevel == "" {
			s.BumpLevel = BumpPatch
		}
	}
	if ys.Tag != nil {
		n++
		prefix := ys.Tag.Prefix
		if prefix == "" {
			prefix = "v"
		}
		s = Step{Kind: StepTag, TagPrefix: prefix, Raw: "tag"}
	}
	if ys.SyncCerts != nil {
		n++
		s = Step{Kind: StepSyncCerts, SyncAction: ys.SyncCerts.Action, EnvRef: ys.SyncCerts.Env, Raw: "sync_certs"}
	}
	if ys.Notify != nil {
		n++
		s = Step{Kind: StepNotify, NotifyVia: ys.Notify.Via, EnvRef: ys.Notify.Env, Raw: "notify"}
	}
	if n != 1 {
		return Step{}, ternerrors.New(ternerrors.ClassConfig, "each YAML step must have exactly one action key")
	}
	return s, nil
}
