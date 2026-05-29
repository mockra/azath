package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/mockra/azath/internal/config"
	"github.com/mockra/azath/internal/copilot"
	"github.com/mockra/azath/internal/project"
	"github.com/mockra/azath/internal/state"
	"github.com/mockra/azath/internal/tmux"
)

const usage = `azath - tmux + Copilot CLI orchestrator

Usage:
  azath up [project] [--fresh]    Start or re-enter a project's House.
                                  No arg opens the dashboard.
  azath down [project] [--kill]   Detach (or kill with --kill).
  azath list [--watch|--plain]    List projects with status.
  azath dash                      Open the dashboard (fzf picker).
  azath pick                      Run the fzf picker once (no tmux session).
  azath restart [--no-build]      Rebuild azath and recreate the dash session.
  azath show <project>            Print project details (used by picker preview).
  azath edit [project]            Open the editor for a project.
  azath resume [project]          Force cold-resume from saved session ID.
  azath config                    Print the merged effective config.
  azath doctor                    Verify dependencies and paths.

Default (no args): azath dash.
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "azath:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return cmdDash()
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "up":
		return cmdUp(rest)
	case "down":
		return cmdDown(rest)
	case "list", "ls":
		return cmdList(rest)
	case "dash":
		return cmdDash()
	case "pick":
		return cmdPick()
	case "restart":
		return cmdRestart(rest)
	case "show":
		return cmdShow(rest)
	case "edit":
		return cmdEdit(rest)
	case "resume":
		return cmdResume(rest)
	case "config":
		return cmdConfig()
	case "doctor":
		return cmdDoctor()
	case "-h", "--help", "help":
		fmt.Print(usage)
		return nil
	default:
		fmt.Print(usage)
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func loadAll() (config.Config, *state.State, []project.Project, error) {
	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		return cfg, nil, nil, err
	}
	st, err := state.Load(cfg.StateFile)
	if err != nil {
		return cfg, nil, nil, err
	}
	projects, err := project.Resolve(cfg)
	if err != nil {
		return cfg, st, nil, err
	}
	return cfg, st, projects, nil
}

func cmdConfig() error {
	cfg, _, projects, err := loadAll()
	if err != nil {
		return err
	}
	fmt.Printf("# config: %s\n", cfg.Path)
	fmt.Printf("agent-command = %q\n", cfg.AgentCommand)
	fmt.Printf("editor = %q\n", cfg.Editor)
	fmt.Printf("editor-placement = %q\n", cfg.EditorPlacement)
	fmt.Printf("projects-root = %v\n", cfg.ProjectsRoot)
	fmt.Printf("auto-discover = %v\n", cfg.AutoDiscover)
	fmt.Printf("state-file = %q\n", cfg.StateFile)
	fmt.Printf("dash-session = %q\n", cfg.DashSession)
	fmt.Println()
	fmt.Println("# Resolved projects:")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tPATH\tAGENT\tEDITOR\tPLACEMENT\tSOURCE")
	for _, p := range projects {
		src := "auto"
		if p.FromConfig {
			src = "config"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", p.Name, p.Path, p.AgentCommand, p.Editor, p.EditorPlacement, src)
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
	live := liveSessions()
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
		mark := "\033[2m○\033[0m"
		if live[sessionName] {
			mark = "\033[32m●\033[0m"
		}
		last := "-"
		if s, ok := st.Get(p.Name); ok && !s.LastUsedAt.IsZero() {
			last = humanize(s.LastUsedAt)
		}
		fmt.Printf("%s  %-*s  %-*s  %s\n", mark, maxName, p.Name, maxPath, p.Path, last)
	}
	return nil
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

func tmuxNameFor(cfg config.Config, project string) string {
	_ = cfg
	return project
}

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
		return cmdDash()
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

	agentCmd := buildAgentCommand(p, resumeID)

	before, _ := copilot.SnapshotSessionIDs()

	if err := tmux.NewDetached(sessionName, p.Path, agentCmd); err != nil {
		return err
	}

	if p.PostStart != "" {
		_ = tmux.Run("new-window", "-t", "="+sessionName, "-n", "post-start", "-c", p.Path, p.PostStart)
	}

	// Give copilot a moment to write its session-state dir.
	go func() {
		time.Sleep(3 * time.Second)
		id, _ := copilot.NewestSessionSince(before)
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

	now := time.Now()
	s, _ := st.Get(p.Name)
	s.TmuxSession = sessionName
	s.StartedAt = now
	s.LastUsedAt = now
	st.Set(p.Name, s)
	_ = st.Save()

	return tmux.AttachOrSwitch(sessionName)
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
		return nil
	}
	if os.Getenv("TMUX") != "" && tmux.CurrentSession() == sessionName {
		return tmux.DetachClient()
	}
	return nil
}

func cmdDash() error {
	cfg, _, _, err := loadAll()
	if err != nil {
		return err
	}
	dash := cfg.DashSession
	if !tmux.HasSession(dash) {
		bin, err := exec.LookPath("azath")
		if err != nil {
			bin = "azath"
		}
		// Loop the picker so the dash session persists across selections.
		cmd := fmt.Sprintf("while %s pick; do :; done; %s pick", bin, bin)
		if err := tmux.NewDetached(dash, os.Getenv("HOME"), cmd); err != nil {
			return err
		}
	}
	return tmux.AttachOrSwitch(dash)
}

func cmdPick() error {
	if _, err := exec.LookPath("fzf"); err != nil {
		return fmt.Errorf("fzf not found in PATH; install fzf to use the picker")
	}
	bin, err := exec.LookPath("azath")
	if err != nil {
		bin = "azath"
	}
	reload := fmt.Sprintf("%s list --plain", bin)
	preview := fmt.Sprintf("%s show {2}", bin)

	for {
		input, err := exec.Command(bin, "list", "--plain").Output()
		if err != nil {
			return fmt.Errorf("list projects: %w", err)
		}
		if len(strings.TrimSpace(string(input))) == 0 {
			fmt.Fprintln(os.Stderr, "azath: no projects discovered")
			return nil
		}

		fzf := exec.Command("fzf",
			"--ansi", "--reverse", "--no-sort", "--no-mouse", "--cycle",
			"--prompt", "azath> ",
			"--header", "enter: up   ^x: kill   ^r: refresh   esc: last session",
			"--delimiter", `[[:space:]]+`, "--nth", "2",
			"--preview", preview,
			"--preview-window", "right:50%:wrap",
			"--bind", fmt.Sprintf("ctrl-x:execute-silent(%s down --kill {2})+reload(%s)", bin, reload),
			"--bind", fmt.Sprintf("ctrl-r:reload(%s)", reload),
			"--bind", "esc:execute-silent(tmux switch-client -l)",
		)
		fzf.Stdin = strings.NewReader(string(input))
		fzf.Stderr = os.Stderr
		out, err := fzf.Output()
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				switch ee.ExitCode() {
				case 130, 1:
					return nil
				}
			}
			return err
		}
		line := strings.TrimSpace(string(out))
		if line == "" {
			return nil
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return nil
		}
		if err := cmdUp([]string{fields[1]}); err != nil {
			fmt.Fprintln(os.Stderr, "azath:", err)
		}
	}
}

func cmdRestart(args []string) error {
	build := true
	for _, a := range args {
		switch a {
		case "--no-build":
			build = false
		default:
			return fmt.Errorf("unknown arg %q", a)
		}
	}

	if build {
		dir, err := sourceDir()
		if err != nil {
			return fmt.Errorf("locate azath source: %w", err)
		}
		fmt.Printf("Building azath in %s...\n", dir)
		cmd := exec.Command("make", "install")
		cmd.Dir = dir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("rebuild: %w", err)
		}
	}

	cfg, _, _, err := loadAll()
	if err != nil {
		return err
	}

	if tmux.CurrentSession() == cfg.DashSession {
		return fmt.Errorf("cannot restart while attached to %q; run from another tmux session or outside tmux", cfg.DashSession)
	}

	if tmux.HasSession(cfg.DashSession) {
		if err := tmux.KillSession(cfg.DashSession); err != nil {
			return err
		}
		fmt.Printf("Killed %s.\n", cfg.DashSession)
	}

	return cmdDash()
}

func sourceDir() (string, error) {
	bin, err := exec.LookPath("azath")
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(bin)
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(real)
	if _, err := os.Stat(filepath.Join(dir, "Makefile")); err != nil {
		return "", fmt.Errorf("no Makefile at %s", dir)
	}
	return dir, nil
}

func cmdShow(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("project name required")
	}
	name := args[0]
	cfg, st, _, err := loadAll()
	if err != nil {
		return err
	}
	p, err := project.Find(cfg, name)
	if err != nil {
		return err
	}
	sessionName := tmuxNameFor(cfg, p.Name)
	running := tmux.HasSession(sessionName)
	status := "stopped"
	if running {
		status = "running"
	}
	fmt.Printf("%s\n", p.Name)
	fmt.Printf("status   %s\n", status)
	fmt.Printf("path     %s\n", p.Path)
	fmt.Printf("agent    %s\n", p.AgentCommand)
	fmt.Printf("editor   %s (%s)\n", p.Editor, p.EditorPlacement)
	if s, ok := st.Get(p.Name); ok {
		if !s.LastUsedAt.IsZero() {
			fmt.Printf("used     %s\n", humanize(s.LastUsedAt))
		}
		if s.CopilotSessionID != "" {
			fmt.Printf("session  %s\n", s.CopilotSessionID)
		}
	}
	if running {
		if wins, err := tmux.ListWindows(sessionName); err == nil && len(wins) > 0 {
			fmt.Println()
			fmt.Println("windows")
			for _, w := range wins {
				fmt.Printf("  %s\n", w)
			}
		}
	}
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

// Compile-time noop to keep imports tidy if we later remove a use.
var _ = sort.Slice
var _ = toml.Decode
