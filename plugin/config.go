package plugin

import (
	"fmt"
	"os"
	"sort"

	"gopkg.in/yaml.v3"
)

// Config is the top-level plugin configuration loaded from YAML.
type Config struct {
	Plugins []PluginConfig `yaml:"plugins" json:"plugins"`
}

// LoadConfigFile reads a YAML config file and returns the parsed Config.
func LoadConfigFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %q: %w", path, err)
	}
	return ParseConfig(data)
}

// ParseConfig parses YAML bytes into a Config.
func ParseConfig(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse plugin config: %w", err)
	}
	return &cfg, nil
}

// EnabledPlugins returns only the enabled plugins, sorted by priority.
func (c *Config) EnabledPlugins() []PluginConfig {
	var out []PluginConfig
	for _, p := range c.Plugins {
		if p.Enabled {
			out = append(out, p)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Priority < out[j].Priority
	})
	return out
}

// Validate checks that all plugin configs have required fields.
func (c *Config) Validate() error {
	names := make(map[string]bool)
	for i, p := range c.Plugins {
		if p.Name == "" {
			return fmt.Errorf("plugin at index %d: name is required", i)
		}
		if names[p.Name] {
			return fmt.Errorf("plugin %q: duplicate name", p.Name)
		}
		names[p.Name] = true
		if p.Type == "" {
			return fmt.Errorf("plugin %q: type is required", p.Name)
		}
	}
	return nil
}
