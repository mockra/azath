package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingReturnsDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AgentCommand != "copilot" {
		t.Errorf("got AgentCommand=%q, want copilot", cfg.AgentCommand)
	}
	if cfg.EditorPlacement != PlacementWindow {
		t.Errorf("got EditorPlacement=%q, want window", cfg.EditorPlacement)
	}
	if !cfg.AutoDiscover {
		t.Error("expected AutoDiscover=true by default")
	}
}

func TestLoadOverridesAndProjects(t *testing.T) {
	t.Setenv("HOME", "/home/test")
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.toml")
	body := `
agent-command = "copilot --foo"
editor = "vim"
editor-placement = "pane-right"
projects-root = ["~/proj"]
auto-discover = false
exclude = ["~/proj/skip"]

[project.alpha]
path = "~/proj/alpha"
agent-command = "copilot"
editor-placement = "pane-bottom"
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.AgentCommand != "copilot --foo" {
		t.Errorf("AgentCommand=%q", cfg.AgentCommand)
	}
	if cfg.Editor != "vim" {
		t.Errorf("Editor=%q", cfg.Editor)
	}
	if cfg.EditorPlacement != PlacementPaneRight {
		t.Errorf("EditorPlacement=%q", cfg.EditorPlacement)
	}
	if cfg.AutoDiscover {
		t.Error("expected AutoDiscover=false")
	}
	if got, want := cfg.ProjectsRoot[0], "/home/test/proj"; got != want {
		t.Errorf("ProjectsRoot[0]=%q, want %q", got, want)
	}
	if got, want := cfg.Exclude[0], "/home/test/proj/skip"; got != want {
		t.Errorf("Exclude[0]=%q, want %q", got, want)
	}
	alpha, ok := cfg.Projects["alpha"]
	if !ok {
		t.Fatal("missing alpha project")
	}
	if alpha.Path != "/home/test/proj/alpha" {
		t.Errorf("alpha.Path=%q", alpha.Path)
	}
}

func TestInvalidPlacementRejected(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "c.toml")
	if err := os.WriteFile(path, []byte(`editor-placement = "weird"`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Error("expected validation error")
	}
}
