package config

import (
	"os"

	ternerrors "github.com/darkmintis/Tern/internal/errors"
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
