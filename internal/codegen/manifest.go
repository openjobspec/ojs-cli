// Package codegen generates type-safe OJS SDK code from job type definitions.
package codegen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// JobTypeDef defines a single job type for code generation.
type JobTypeDef struct {
	Type        string            `yaml:"type" json:"type"`
	Description string            `yaml:"description" json:"description"`
	Queue       string            `yaml:"queue" json:"queue"`
	Args        []ArgDef          `yaml:"args" json:"args"`
	Retry       *RetryDef         `yaml:"retry,omitempty" json:"retry,omitempty"`
	Unique      *UniqueDef        `yaml:"unique,omitempty" json:"unique,omitempty"`
	Timeout     int               `yaml:"timeout_ms,omitempty" json:"timeout_ms,omitempty"`
	Priority    *int              `yaml:"priority,omitempty" json:"priority,omitempty"`
	Tags        []string          `yaml:"tags,omitempty" json:"tags,omitempty"`
	Meta        map[string]string `yaml:"meta,omitempty" json:"meta,omitempty"`
}

// ArgDef defines a single argument.
type ArgDef struct {
	Name        string `yaml:"name" json:"name"`
	Type        string `yaml:"type" json:"type"` // string, int, float, bool, object, array
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Required    bool   `yaml:"required" json:"required"`
}

// RetryDef defines retry policy.
type RetryDef struct {
	MaxAttempts int    `yaml:"max_attempts" json:"max_attempts"`
	Backoff     string `yaml:"backoff" json:"backoff"` // exponential, linear, constant
	InitialMs   int    `yaml:"initial_ms" json:"initial_ms"`
}

// UniqueDef defines unique job constraints.
type UniqueDef struct {
	Key    string `yaml:"key" json:"key"`
	Period string `yaml:"period" json:"period"` // e.g., "5m", "1h"
}

// Manifest holds a collection of job type definitions.
type Manifest struct {
	Version  string       `yaml:"version" json:"version"`
	Package  string       `yaml:"package" json:"package"`
	JobTypes []JobTypeDef `yaml:"job_types" json:"job_types"`
}

// LoadManifest reads a manifest from a YAML or JSON file.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading manifest %s: %w", path, err)
	}

	var manifest Manifest
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &manifest); err != nil {
			return nil, fmt.Errorf("parsing YAML manifest: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, &manifest); err != nil {
			return nil, fmt.Errorf("parsing JSON manifest: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported manifest format: %s (use .yaml or .json)", ext)
	}

	if err := validateManifest(&manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

func validateManifest(m *Manifest) error {
	if len(m.JobTypes) == 0 {
		return fmt.Errorf("manifest contains no job types")
	}
	seen := make(map[string]bool)
	for i, jt := range m.JobTypes {
		if jt.Type == "" {
			return fmt.Errorf("job_types[%d]: type is required", i)
		}
		if seen[jt.Type] {
			return fmt.Errorf("duplicate job type: %s", jt.Type)
		}
		seen[jt.Type] = true
		if jt.Queue == "" {
			m.JobTypes[i].Queue = "default"
		}
		for j, arg := range jt.Args {
			if arg.Name == "" {
				return fmt.Errorf("job_types[%d].args[%d]: name is required", i, j)
			}
			if arg.Type == "" {
				m.JobTypes[i].Args[j].Type = "string"
			}
		}
	}
	return nil
}
