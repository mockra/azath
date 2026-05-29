package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mockra/azath/internal/config"
	"github.com/mockra/azath/internal/copilot"
	"github.com/mockra/azath/internal/project"
	"github.com/mockra/azath/internal/state"
	"github.com/mockra/azath/internal/tmux"
)

type upOpts struct {
	fresh bool
}

func cmdUp(args []string) error {
	var name string
	opts := upOpts{}
	for _, a := range args {
		switch {
		case a == "--fresh":
			opts.fresh = true
		case strings.HasPrefix(a, "-"):
			return fmt.Errorf("unknown flag %q", a)
		default:
			if name != "" {
				return fmt.Errorf("only one project at a time")
			}
			name = a
		}
	}
	if name == "" {
		return cmdDash(nil)
	}

	cfg, st, _, err := loadAll()
	if err != nil {
		return err
	}
	p, err := project.Find(cfg, name)
	if err != nil {
		return err
	}
	sessionName := tmuxNameFor(cfg, p.Name)

	if tmux.HasSession(sessionName) {
		s, _ := st.Get(p.Name)
		s.LastUsedAt = time.Now()
		st.Set(p.Name, s)
		_ = st.Save()
		return tmux.AttachOrSwitch(sessionName)
	}

	resumeID := ""
	if !opts.fresh {
		if s, ok := st.Get(p.Name); ok {
			resumeID = s.CopilotSessionID
		}
	}

	startCmd, startsAgent := buildStartCommand(p, resumeID)

	var before map[string]bool
	copilotDir := cfg.CopilotSessionDir
	if startsAgent && copilotDir != "" {
		before, _ = copilot.SnapshotSessionIDsDir(copilotDir)
	}

	if err := tmux.NewDetached(sessionName, p.Path, startCmd); err != nil {
		return err
	}

	if p.PostStart != "" {
		_ = tmux.Run("new-window", "-t", "="+sessionName, "-n", "post-start", "-c", p.Path, p.PostStart)
	}

	if startsAgent && copilotDir != "" {
		// Give copilot a moment to write its session-state dir.
		go func() {
			time.Sleep(3 * time.Second)
			id, _ := copilot.NewestSessionSinceDir(before, copilotDir)
			if id == "" {
				return
			}
			st2, err := state.Load(cfg.StateFile)
			if err != nil {
				return
			}
			s, _ := st2.Get(p.Name)
			s.TmuxSession = sessionName
			s.CopilotSessionID = id
			if s.StartedAt.IsZero() {
				s.StartedAt = time.Now()
			}
			s.LastUsedAt = time.Now()
			st2.Set(p.Name, s)
			_ = st2.Save()
		}()
	}

	now := time.Now()
	s, _ := st.Get(p.Name)
	s.TmuxSession = sessionName
	s.StartedAt = now
	s.LastUsedAt = now
	st.Set(p.Name, s)
	_ = st.Save()

	return tmux.AttachOrSwitch(sessionName)
}

// buildStartCommand returns the shell command to run as the project session's
// first window, and whether that command launches the agent (used to gate
// copilot session-ID capture).
func buildStartCommand(p project.Project, resumeID string) (string, bool) {
	switch p.StartWith {
	case config.StartWithShell:
		return "", false
	case config.StartWithEditor:
		if strings.TrimSpace(p.Editor) == "" {
			return "", false
		}
		return p.Editor + " .", false
	default:
		return buildAgentCommand(p, resumeID), true
	}
}

func buildAgentCommand(p project.Project, resumeID string) string {
	base := strings.TrimSpace(p.AgentCommand)
	if base == "" {
		return ""
	}
	if resumeID == "" {
		return base
	}
	// Inject --resume=<id> only for the bare copilot command. For wrappers
	// like `github-dev`, we cannot safely add CLI flags.
	if isCopilotCommand(base) {
		return base + " --resume=" + resumeID
	}
	return base
}

func isCopilotCommand(cmd string) bool {
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return false
	}
	return fields[0] == "copilot"
}

type downOpts struct {
	kill bool
}

func cmdDown(args []string) error {
	var name string
	opts := downOpts{}
	for _, a := range args {
		switch {
		case a == "--kill":
			opts.kill = true
		case strings.HasPrefix(a, "-"):
			return fmt.Errorf("unknown flag %q", a)
		default:
			name = a
		}
	}
	cfg, st, _, err := loadAll()
	if err != nil {
		return err
	}
	if name == "" {
		name = tmux.CurrentSession()
		if name == "" {
			return fmt.Errorf("no project specified and not inside a tmux session")
		}
	}
	sessionName := tmuxNameFor(cfg, name)
	if !tmux.HasSession(sessionName) {
		return fmt.Errorf("no running session for %q", name)
	}
	if opts.kill {
		if err := tmux.KillSession(sessionName); err != nil {
			return err
		}
		s, ok := st.Get(name)
		if ok {
			s.TmuxSession = ""
			st.Set(name, s)
			_ = st.Save()
		}
		fmt.Printf("killed %s\n", sessionName)
		return nil
	}
	// Detach: if we're inside the target, detach the local client; otherwise
	// detach all clients attached to that session (no-op if none, but no
	// longer silent — surface a clear message).
	if os.Getenv("TMUX") != "" && tmux.CurrentSession() == sessionName {
		return tmux.DetachClient()
	}
	if err := tmux.DetachSession(sessionName); err != nil {
		return err
	}
	fmt.Printf("detached clients from %s (session still running; use --kill to stop)\n", sessionName)
	return nil
}

func cmdEdit(args []string) error {
	cfg, _, _, err := loadAll()
	if err != nil {
		return err
	}
	var name string
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			return fmt.Errorf("unknown flag %q", a)
		}
		name = a
	}
	if name == "" {
		name = tmux.CurrentSession()
		if name == "" || name == cfg.DashSession {
			return fmt.Errorf("no project specified and not inside a project session")
		}
	}
	p, err := project.Find(cfg, name)
	if err != nil {
		return err
	}
	sessionName := tmuxNameFor(cfg, p.Name)
	if !tmux.HasSession(sessionName) {
		return fmt.Errorf("session %q not running; run `azath up %s` first", sessionName, p.Name)
	}

	editorCmd := fmt.Sprintf("%s .", p.Editor)

	switch p.EditorPlacement {
	case config.PlacementWindow:
		windows, _ := tmux.ListWindows(sessionName)
		for _, w := range windows {
			if w == "editor" {
				return tmux.SelectWindow(sessionName + ":editor")
			}
		}
		return tmux.NewWindow(sessionName, "editor", p.Path, editorCmd)
	case config.PlacementPaneRight:
		return tmux.SplitWindow(sessionName+":1", true, p.Path, editorCmd)
	case config.PlacementPaneBottom:
		return tmux.SplitWindow(sessionName+":1", false, p.Path, editorCmd)
	default:
		return fmt.Errorf("unsupported placement %q", p.EditorPlacement)
	}
}

func cmdResume(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: azath resume <project>")
	}
	cfg, st, _, err := loadAll()
	if err != nil {
		return err
	}
	name := args[0]
	p, err := project.Find(cfg, name)
	if err != nil {
		return err
	}
	s, ok := st.Get(p.Name)
	if !ok || s.CopilotSessionID == "" {
		return fmt.Errorf("no saved copilot session id for %q", name)
	}
	sessionName := tmuxNameFor(cfg, p.Name)
	if tmux.HasSession(sessionName) {
		return fmt.Errorf("session %q already running; use `azath down --kill %s` first", sessionName, p.Name)
	}
	agentCmd := buildAgentCommand(p, s.CopilotSessionID)
	if err := tmux.NewDetached(sessionName, p.Path, agentCmd); err != nil {
		return err
	}
	s.TmuxSession = sessionName
	s.LastUsedAt = time.Now()
	st.Set(p.Name, s)
	_ = st.Save()
	return tmux.AttachOrSwitch(sessionName)
}
