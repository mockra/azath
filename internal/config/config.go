package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type Placement string

const (
	PlacementWindow      Placement = "window"
	PlacementPaneRight   Placement = "pane-right"
	PlacementPaneBottom  Placement = "pane-bottom"
)

type Project struct {
	Path            string `toml:"path"`
	AgentCommand    string `toml:"agent-command"`
	Editor          string `toml:"editor"`
	EditorPlacement string `toml:"editor-placement"`
	PostStart       string `toml:"post-start"`
}

type raw struct {
	AgentCommand    string             `toml:"agent-command"`
	Editor          string             `toml:"editor"`
	EditorPlacement string             `toml:"editor-placement"`
	ProjectsRoot    []string           `toml:"projects-root"`
	AutoDiscover    *bool              `toml:"auto-discover"`
	Exclude         []string           `toml:"exclude"`
	StateFile       string             `toml:"state-file"`
	DashSession     string             `toml:"dash-session"`
	Project         map[string]Project `toml:"project"`
}

type Config struct {
	AgentCommand    string
	Editor          string
	EditorPlacement Placement
	ProjectsRoot    []string
	AutoDiscover    bool
	Exclude         []string
	StateFile       string
	DashSession     string
	Projects        map[string]Project
	Path            string
}

func DefaultPath() string {
	if x := os.Getenv("AZATH_CONFIG"); x != "" {
		return x
	}
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "azath", "config.toml")
	}
	return filepath.Join(os.Getenv("HOME"), ".config", "azath", "config.toml")
}

func defaults() Config {
	home := os.Getenv("HOME")
	return Config{
		AgentCommand:    "copilot",
		Editor:          "nvim",
		EditorPlacement: PlacementWindow,
		AutoDiscover:    true,
		StateFile:       filepath.Join(home, ".local", "share", "azath", "state.toml"),
		DashSession:     "azath-dash",
		Projects:        map[string]Project{},
	}
}

func Load(path string) (Config, error) {
	cfg := defaults()
	cfg.Path = path

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}

	var r raw
	if _, err := toml.Decode(string(data), &r); err != nil {
		return cfg, fmt.Errorf("parse %s: %w", path, err)
	}

	if r.AgentCommand != "" {
		cfg.AgentCommand = r.AgentCommand
	}
	if r.Editor != "" {
		cfg.Editor = r.Editor
	}
	if r.EditorPlacement != "" {
		cfg.EditorPlacement = Placement(r.EditorPlacement)
	}
	if len(r.ProjectsRoot) > 0 {
		cfg.ProjectsRoot = r.ProjectsRoot
	}
	if r.AutoDiscover != nil {
		cfg.AutoDiscover = *r.AutoDiscover
	}
	if len(r.Exclude) > 0 {
		cfg.Exclude = r.Exclude
	}
	if r.StateFile != "" {
		cfg.StateFile = r.StateFile
	}
	if r.DashSession != "" {
		cfg.DashSession = r.DashSession
	}
	if r.Project != nil {
		cfg.Projects = r.Project
	}

	cfg.expandPaths()
	if err := cfg.validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (c *Config) expandPaths() {
	for i, p := range c.ProjectsRoot {
		c.ProjectsRoot[i] = expand(p)
	}
	for i, p := range c.Exclude {
		c.Exclude[i] = expand(p)
	}
	c.StateFile = expand(c.StateFile)
	out := map[string]Project{}
	for name, p := range c.Projects {
		p.Path = expand(p.Path)
		out[name] = p
	}
	c.Projects = out
}

func (c *Config) validate() error {
	switch c.EditorPlacement {
	case PlacementWindow, PlacementPaneRight, PlacementPaneBottom:
	default:
		return fmt.Errorf("invalid editor-placement %q (want: window, pane-right, pane-bottom)", c.EditorPlacement)
	}
	for name, p := range c.Projects {
		if p.Path == "" {
			return fmt.Errorf("project %q is missing path", name)
		}
	}
	return nil
}

func expand(p string) string {
	if p == "" {
		return p
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(os.Getenv("HOME"), p[2:])
	}
	if p == "~" {
		return os.Getenv("HOME")
	}
	return os.ExpandEnv(p)
}
