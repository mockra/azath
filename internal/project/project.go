package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/mockra/azath/internal/config"
)

type Project struct {
	Name            string
	Path            string
	AgentCommand    string
	Editor          string
	EditorPlacement config.Placement
	PostStart       string
	FromConfig      bool
}

func Resolve(cfg config.Config) ([]Project, error) {
	byName := map[string]Project{}

	for name, p := range cfg.Projects {
		byName[name] = merge(name, p, cfg, true)
	}

	if cfg.AutoDiscover {
		excludes := map[string]bool{}
		for _, e := range cfg.Exclude {
			excludes[filepath.Clean(e)] = true
		}
		for _, root := range cfg.ProjectsRoot {
			entries, err := os.ReadDir(root)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, fmt.Errorf("scan %s: %w", root, err)
			}
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				full := filepath.Join(root, e.Name())
				if excludes[filepath.Clean(full)] {
					continue
				}
				if _, err := os.Stat(filepath.Join(full, ".git")); err != nil {
					continue
				}
				if _, ok := byName[e.Name()]; ok {
					continue
				}
				byName[e.Name()] = merge(e.Name(), config.Project{Path: full}, cfg, false)
			}
		}
	}

	out := make([]Project, 0, len(byName))
	for _, p := range byName {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func Find(cfg config.Config, name string) (Project, error) {
	all, err := Resolve(cfg)
	if err != nil {
		return Project{}, err
	}
	for _, p := range all {
		if p.Name == name {
			return p, nil
		}
	}
	return Project{}, fmt.Errorf("unknown project %q", name)
}

func merge(name string, p config.Project, cfg config.Config, fromConfig bool) Project {
	out := Project{
		Name:            name,
		Path:            p.Path,
		AgentCommand:    p.AgentCommand,
		Editor:          p.Editor,
		EditorPlacement: config.Placement(p.EditorPlacement),
		PostStart:       p.PostStart,
		FromConfig:      fromConfig,
	}
	if out.AgentCommand == "" {
		out.AgentCommand = cfg.AgentCommand
	}
	if out.Editor == "" {
		out.Editor = cfg.Editor
	}
	if out.EditorPlacement == "" {
		out.EditorPlacement = cfg.EditorPlacement
	}
	return out
}
