package config

import (
	"fmt"

	ternerrors "github.com/darkmintis/Tern/internal/errors"
	"gopkg.in/yaml.v3"
)

// yamlFile is the structured escape hatch.
type yamlFile struct {
	Lanes map[string][]yamlStep `yaml:"lanes"`
}

type yamlStep struct {
	Build     *yamlBuild     `yaml:"build"`
	Sign      *yamlSign      `yaml:"sign"`
	Upload    *yamlUpload    `yaml:"upload"`
	Ship      *yamlShip      `yaml:"ship"`
	Bump      *yamlBump      `yaml:"bump"`
	Tag       *yamlTag       `yaml:"tag"`
	SyncCerts *yamlSyncCerts `yaml:"sync_certs"`
	Notify    *yamlNotify    `yaml:"notify"`
}

type yamlBuild struct {
	Platform string `yaml:"platform"`
	Mode     string `yaml:"mode"`
	Kind     string `yaml:"kind"` // aab | apk
	Flavor   string `yaml:"flavor"`
	Scheme   string `yaml:"scheme"`
}

type yamlShip struct {
	Platform    string  `yaml:"platform"`
	From        string  `yaml:"from"`
	To          string  `yaml:"to"`
	Track       string  `yaml:"track"`
	Rollout     float64 `yaml:"rollout"` // fraction 0–1 or percent >1 (normalized in apply)
	ReleaseName string  `yaml:"release_name"`
	Notes       string  `yaml:"notes"`
	NotesFile   string  `yaml:"notes_file"`
	NotesLocale string  `yaml:"notes_locale"`
}

type yamlSign struct {
	Platform string `yaml:"platform"`
	With     string `yaml:"with"`
	Env      string `yaml:"env"`
}

type yamlUpload struct {
	Platform    string  `yaml:"platform"`
	To          string  `yaml:"to"`
	Track       string  `yaml:"track"`
	Rollout     float64 `yaml:"rollout"`
	ReleaseName string  `yaml:"release_name"`
	Notes       string  `yaml:"notes"`
	NotesFile   string  `yaml:"notes_file"`
	NotesLocale string  `yaml:"notes_locale"`
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
		kind := ArtifactKind(ys.Build.Kind)
		if kind == "" && p == PlatformAndroid && m == ModeRelease {
			kind = ArtifactAAB
		}
		s = Step{
			Kind: StepBuild, Platform: p, Mode: m, ArtifactKind: kind,
			Flavor: ys.Build.Flavor, Scheme: ys.Build.Scheme, Raw: "build",
		}
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
		if err := applyYAMLRollout(&s, ys.Upload.Rollout); err != nil {
			return Step{}, err
		}
		if err := applyYAMLReleaseMeta(&s, ys.Upload.ReleaseName, ys.Upload.Notes, ys.Upload.NotesFile, ys.Upload.NotesLocale); err != nil {
			return Step{}, err
		}
	}
	if ys.Ship != nil {
		n++
		p, err := parsePlatform(ys.Ship.Platform)
		if err != nil {
			return Step{}, err
		}
		from := ys.Ship.From
		if from == "" {
			from = "last"
		}
		s = Step{Kind: StepShip, Platform: p, ShipFrom: from, UploadTarget: ys.Ship.To, Track: ys.Ship.Track, Raw: "ship"}
		if err := applyYAMLRollout(&s, ys.Ship.Rollout); err != nil {
			return Step{}, err
		}
		if err := applyYAMLReleaseMeta(&s, ys.Ship.ReleaseName, ys.Ship.Notes, ys.Ship.NotesFile, ys.Ship.NotesLocale); err != nil {
			return Step{}, err
		}
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

func applyYAMLRollout(s *Step, rollout float64) error {
	if rollout == 0 {
		return nil
	}
	// YAML may use 10 for 10% or 0.1 for fraction.
	v := fmt.Sprintf("%g", rollout)
	frac, err := parseRolloutValue(v)
	if err != nil {
		return err
	}
	s.Rollout = frac
	return nil
}

func applyYAMLReleaseMeta(s *Step, releaseName, notes, notesFile, notesLocale string) error {
	if releaseName != "" {
		strategy, custom, err := parseReleaseNameValue(releaseName)
		if err != nil {
			return err
		}
		s.ReleaseNameStrategy = strategy
		s.ReleaseNameCustom = custom
	}
	if notesFile != "" {
		s.NotesMode = "file"
		s.NotesFile = notesFile
	} else if notes != "" {
		mode, text, file, err := parseNotesValue(notes)
		if err != nil {
			return err
		}
		s.NotesMode = mode
		s.NotesText = text
		s.NotesFile = file
	}
	if notesLocale != "" {
		s.NotesLocale = notesLocale
	}
	return nil
}
