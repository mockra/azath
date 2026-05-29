package tmux

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

func Run(args ...string) error {
	cmd := exec.Command("tmux", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Output(args ...string) (string, error) {
	cmd := exec.Command("tmux", args...)
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return "", fmt.Errorf("tmux %s: %s", strings.Join(args, " "), strings.TrimSpace(string(ee.Stderr)))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func HasSession(name string) bool {
	cmd := exec.Command("tmux", "has-session", "-t", "="+name)
	return cmd.Run() == nil
}

func NewDetached(name, dir, cmd string) error {
	args := []string{"new-session", "-d", "-s", name, "-c", dir}
	if cmd != "" {
		args = append(args, cmd)
	}
	return Run(args...)
}

func NewWindow(target, name, dir, cmd string) error {
	args := []string{"new-window", "-t", target, "-n", name, "-c", dir}
	if cmd != "" {
		args = append(args, cmd)
	}
	return Run(args...)
}

func SplitWindow(target string, horizontal bool, dir, cmd string) error {
	args := []string{"split-window", "-t", target, "-c", dir}
	if horizontal {
		args = append(args, "-h")
	} else {
		args = append(args, "-v")
	}
	if cmd != "" {
		args = append(args, cmd)
	}
	return Run(args...)
}

func KillSession(name string) error {
	return Run("kill-session", "-t", "="+name)
}

func ListSessions() ([]string, error) {
	out, err := Output("list-sessions", "-F", "#{session_name}")
	if err != nil {
		if strings.Contains(err.Error(), "no server running") {
			return nil, nil
		}
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

func ListWindows(session string) ([]string, error) {
	out, err := Output("list-windows", "-t", "="+session, "-F", "#{window_name}")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

func SelectWindow(target string) error {
	return Run("select-window", "-t", target)
}

// AttachOrSwitch attaches to a session, or switches client if already inside tmux.
// This replaces the current process with tmux when not already inside tmux.
func AttachOrSwitch(name string) error {
	if os.Getenv("TMUX") != "" {
		return Run("switch-client", "-t", "="+name)
	}
	bin, err := exec.LookPath("tmux")
	if err != nil {
		return err
	}
	return syscall.Exec(bin, []string{"tmux", "attach-session", "-t", "=" + name}, os.Environ())
}

func DetachClient() error {
	if os.Getenv("TMUX") == "" {
		return errors.New("not inside a tmux session")
	}
	return Run("detach-client")
}

// DetachSession detaches all clients attached to the named session.
func DetachSession(name string) error {
	return Run("detach-client", "-s", "="+name)
}

// CapturePane returns the contents of the named target pane (e.g. "session" or
// "session:window.pane"). lines is the number of trailing lines to include.
func CapturePane(target string, lines int) (string, error) {
	if lines <= 0 {
		lines = 200
	}
	return Output("capture-pane", "-t", target, "-p", "-S", fmt.Sprintf("-%d", lines))
}

// PaneHash returns a short hex fingerprint of the trailing pane content.
// Returns "" without error when the session/pane is missing.
func PaneHash(target string, lines int) string {
	out, err := CapturePane(target, lines)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256([]byte(out))
	return hex.EncodeToString(sum[:8])
}

// CurrentSession returns the name of the tmux session the caller is in, or "".
func CurrentSession() string {
	if os.Getenv("TMUX") == "" {
		return ""
	}
	out, err := Output("display-message", "-p", "#{session_name}")
	if err != nil {
		return ""
	}
	return out
}
