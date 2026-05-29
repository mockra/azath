package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/mockra/azath/internal/config"
	"github.com/mockra/azath/internal/project"
	"github.com/mockra/azath/internal/state"
	"github.com/mockra/azath/internal/tmux"
)

func cmdConfig() error {
	cfg, _, projects, err := loadAll()
	if err != nil {
		return err
	}
	fmt.Printf("# config: %s\n", cfg.Path)
	fmt.Printf("agent-command       = %q\n", cfg.AgentCommand)
	fmt.Printf("editor              = %q\n", cfg.Editor)
	fmt.Printf("editor-placement    = %q\n", cfg.EditorPlacement)
	fmt.Printf("start-with          = %q\n", cfg.StartWith)
	fmt.Printf("projects-root       = %v\n", cfg.ProjectsRoot)
	fmt.Printf("auto-discover       = %v\n", cfg.AutoDiscover)
	fmt.Printf("state-file          = %q\n", cfg.StateFile)
	fmt.Printf("dash-session        = %q\n", cfg.DashSession)
	fmt.Printf("session-prefix      = %q\n", cfg.SessionPrefix)
	fmt.Printf("copilot-session-dir = %q\n", cfg.CopilotSessionDir)
	fmt.Printf("idle-timeout        = %q\n", cfg.IdleTimeout)
	fmt.Println()
	fmt.Println("# Resolved projects:")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tPATH\tAGENT\tEDITOR\tPLACEMENT\tSTART\tSOURCE")
	for _, p := range projects {
		src := "auto"
		if p.FromConfig {
			src = "config"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", p.Name, p.Path, p.AgentCommand, p.Editor, p.EditorPlacement, p.StartWith, src)
	}
	return w.Flush()
}

func cmdDoctor() error {
	ok := true
	check := func(name string, condition bool, detail string) {
		marker := "ok"
		if !condition {
			marker = "FAIL"
			ok = false
		}
		fmt.Printf("[%s] %s: %s\n", marker, name, detail)
	}

	for _, bin := range []string{"tmux", "copilot", "nvim", "fzf"} {
		path, err := exec.LookPath(bin)
		if err != nil {
			check(bin, false, "not found in PATH")
		} else {
			check(bin, true, path)
		}
	}

	cfg, _, _, err := loadAll()
	if err != nil {
		check("config", false, err.Error())
		return nil
	}
	check("config", true, cfg.Path)
	for _, r := range cfg.ProjectsRoot {
		_, err := os.Stat(r)
		check("projects-root: "+r, err == nil, statusOrError(r, err))
	}
	check("state-file", canWrite(cfg.StateFile), cfg.StateFile)
	if cfg.CopilotSessionDir != "" {
		if _, err := os.Stat(cfg.CopilotSessionDir); err != nil {
			fmt.Printf("[warn] copilot-session-dir: %s (%v) — resume IDs will not be captured\n", cfg.CopilotSessionDir, err)
		} else {
			check("copilot-session-dir", true, cfg.CopilotSessionDir)
		}
	}

	if !ok {
		return fmt.Errorf("doctor reported failures")
	}
	return nil
}

func statusOrError(_ string, err error) string {
	if err != nil {
		return err.Error()
	}
	return "exists"
}

func canWrite(path string) bool {
	dir := pathDir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false
	}
	f, err := os.OpenFile(path+".doctor", os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(path + ".doctor")
	return true
}

func pathDir(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[:i]
		}
	}
	return "."
}

func cmdList(args []string) error {
	watch := false
	plain := false
	for _, a := range args {
		switch a {
		case "--watch", "-w":
			watch = true
		case "--plain":
			plain = true
		}
	}
	if plain {
		return printListPlain()
	}
	if !watch {
		return printList()
	}
	for {
		fmt.Print("\033[H\033[2J")
		fmt.Printf("azath %s\n\n", time.Now().Format(time.RFC3339))
		if err := printList(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		time.Sleep(2 * time.Second)
	}
}

func printList() error {
	cfg, st, projects, err := loadAll()
	if err != nil {
		return err
	}
	pruneStaleState(st, projectNames(projects))
	live := liveSessions()
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PROJECT\tSTATUS\tPATH\tLAST USED")
	for _, p := range projects {
		status := "stopped"
		sessionName := tmuxNameFor(cfg, p.Name)
		if live[sessionName] {
			status = "running"
		}
		last := "-"
		if s, ok := st.Get(p.Name); ok && !s.LastUsedAt.IsZero() {
			last = humanize(s.LastUsedAt)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.Name, status, p.Path, last)
	}
	return w.Flush()
}

func printListPlain() error {
	cfg, st, projects, err := loadAll()
	if err != nil {
		return err
	}
	pruneStaleState(st, projectNames(projects))
	live := liveSessions()
	sort.SliceStable(projects, func(i, j int) bool {
		ri := live[tmuxNameFor(cfg, projects[i].Name)]
		rj := live[tmuxNameFor(cfg, projects[j].Name)]
		if ri != rj {
			return ri
		}
		return projects[i].Name < projects[j].Name
	})
	maxName := 0
	for _, p := range projects {
		if n := len(p.Name); n > maxName {
			maxName = n
		}
	}
	maxPath := 0
	for _, p := range projects {
		if n := len(p.Path); n > maxPath {
			maxPath = n
		}
	}
	for _, p := range projects {
		sessionName := tmuxNameFor(cfg, p.Name)
		info := collectDashInfo(cfg, st, p, sessionName, live[sessionName])
		mark := markGlyph(info.activity)
		if u := unreadGlyph(info.unread); u != "" {
			// Attach the unread bell directly to the mark so fzf still
			// sees a single status field.
			mark = mark + u
		}
		last := "-"
		if s, ok := st.Get(p.Name); ok && !s.LastUsedAt.IsZero() {
			last = humanize(s.LastUsedAt)
		}
		row := fmt.Sprintf("%s  %-*s  %-*s  %s", mark, maxName, p.Name, maxPath, p.Path, last)
		if info.lastMessage != "" {
			// Dim the trailing summary so it doesn't dominate the row.
			row += fmt.Sprintf("  \033[2m| %s\033[0m", truncateRunes(info.lastMessage, 60))
		}
		fmt.Println(row)
	}
	return nil
}

// truncateRunes shortens s to at most max runes, appending an ellipsis when cut.
func truncateRunes(s string, max int) string {
	if max <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}

// pruneStaleState removes state entries for projects that no longer exist.
// Failures are ignored so list/picker stay responsive on read-only state.
func pruneStaleState(st *state.State, names []string) {
	if removed := st.Prune(names); len(removed) > 0 {
		_ = st.Save()
	}
}

func projectNames(projects []project.Project) []string {
	out := make([]string, 0, len(projects))
	for _, p := range projects {
		out = append(out, p.Name)
	}
	return out
}

func liveSessions() map[string]bool {
	live := map[string]bool{}
	if sessions, err := tmux.ListSessions(); err == nil {
		for _, s := range sessions {
			live[s] = true
		}
	}
	return live
}

func humanize(t time.Time) string {
	d := time.Since(t).Round(time.Second)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// tmuxNameFor builds the tmux session name for a project, honouring an
// optional session-prefix from config (e.g. "azath/web" instead of just "web")
// so dashboards can scope tmux session listings.
func tmuxNameFor(cfg config.Config, project string) string {
	if cfg.SessionPrefix == "" {
		return project
	}
	return cfg.SessionPrefix + "/" + project
}
