package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mockra/azath/internal/config"
)

func TestResolveAutoDiscoverAndOverride(t *testing.T) {
	tmp := t.TempDir()
	mkRepo := func(name string) string {
		p := filepath.Join(tmp, name)
		if err := os.MkdirAll(filepath.Join(p, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	mkRepo("alpha")
	mkRepo("beta")
	mkRepo("skipme")
	// non-git dir is ignored
	if err := os.MkdirAll(filepath.Join(tmp, "notgit"), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		AgentCommand:    "copilot",
		Editor:          "nvim",
		EditorPlacement: config.PlacementWindow,
		ProjectsRoot:    []string{tmp},
		AutoDiscover:    true,
		Exclude:         []string{filepath.Join(tmp, "skipme")},
		Projects: map[string]config.Project{
			"alpha": {
				Path:         filepath.Join(tmp, "alpha"),
				AgentCommand: "copilot --override",
			},
		},
	}

	got, err := Resolve(cfg)
	if err != nil {
		t.Fatal(err)
	}

	names := []string{}
	for _, p := range got {
		names = append(names, p.Name)
	}
	if len(got) != 2 {
		t.Fatalf("got %d projects (%v), want 2 (alpha, beta)", len(got), names)
	}
	if got[0].Name != "alpha" || got[1].Name != "beta" {
		t.Errorf("ordering wrong: %v", names)
	}
	if got[0].AgentCommand != "copilot --override" {
		t.Errorf("alpha override not applied: %q", got[0].AgentCommand)
	}
	if !got[0].FromConfig {
		t.Errorf("alpha should be FromConfig=true")
	}
	if got[1].AgentCommand != "copilot" {
		t.Errorf("beta should inherit default agent: %q", got[1].AgentCommand)
	}
	if got[1].FromConfig {
		t.Errorf("beta should be FromConfig=false (auto)")
	}
}
