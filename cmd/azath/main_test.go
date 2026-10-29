package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mockra/azath/internal/state"
)

const savedSessionID = "9da1ad1f-173f-4efb-99d1-a7cb43842304"

var sessionIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestUpPersistsSessionBeforeLaunch(t *testing.T) {
	tests := []struct {
		name       string
		project    string
		previousID string
		fresh      bool
		copilot    bool
		newID      bool
		command    string
	}{
		{name: "new Copilot session", copilot: true, newID: true},
		{name: "resume saved session", previousID: savedSessionID, copilot: true},
		{name: "fresh replaces saved session", previousID: savedSessionID, fresh: true, copilot: true, newID: true},
		{name: "Copilot arguments", project: `agent-command = "copilot --model auto"`, copilot: true, newID: true, command: "copilot --model auto"},
		{name: "wrapper", project: `agent-command = "github-dev"`, previousID: savedSessionID, command: "github-dev"},
		{name: "shell", project: `start-with = "shell"`, previousID: savedSessionID},
		{name: "editor", project: "start-with = \"editor\"\neditor = \"nvim\"", previousID: savedSessionID, command: "nvim ."},
		{name: "command", project: `command = "oponn"`, previousID: savedSessionID, command: "oponn"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statePath, launchStatePath, logPath := setupProject(t, tt.project, tt.previousID)
			args := []string{"demo"}
			if tt.fresh {
				args = append(args, "--fresh")
			}
			if err := cmdUp(args); err != nil {
				t.Fatal(err)
			}
			launched := loadSession(t, launchStatePath)
			persisted := loadSession(t, statePath)
			if launched != persisted {
				t.Fatalf("state at launch = %+v, final state = %+v", launched, persisted)
			}
			if launched.TmuxSession != "demo" || launched.StartedAt.IsZero() || launched.LastUsedAt.IsZero() {
				t.Fatalf("incomplete state at launch: %+v", launched)
			}
			if tt.newID {
				if !sessionIDPattern.MatchString(launched.CopilotSessionID) || launched.CopilotSessionID == tt.previousID {
					t.Fatalf("expected new UUID, got %q", launched.CopilotSessionID)
				}
			} else if launched.CopilotSessionID != tt.previousID {
				t.Fatalf("session ID = %q, want %q", launched.CopilotSessionID, tt.previousID)
			}

			command := tt.command
			if tt.copilot {
				if command == "" {
					command = "copilot"
				}
				command += " --session-id=" + launched.CopilotSessionID
			}
			want := "has-session\n-t\n=demo\nnew-session\n-d\n-s\ndemo\n-c\n" + filepath.Dir(statePath) + "\n"
			if command != "" {
				want += command + "\n"
			}
			want += "switch-client\n-t\n=demo\n"
			if got := readFile(t, logPath); got != want {
				t.Fatalf("tmux calls = %q, want %q", got, want)
			}
		})
	}
}

func TestUpSaveFailurePreventsLaunch(t *testing.T) {
	_, _, logPath := setupProject(t, "", "")
	t.Setenv("AZATH_TEST_BLOCK_SAVE", "1")
	if err := cmdUp([]string{"demo"}); err == nil || !strings.Contains(err.Error(), "save project state") {
		t.Fatalf("expected save error, got %v", err)
	}
	if got := readFile(t, logPath); got != "has-session\n-t\n=demo\n" {
		t.Fatalf("unexpected tmux calls after save failure: %q", got)
	}
}

func TestUpLaunchFailureRetainsSessionID(t *testing.T) {
	statePath, launchStatePath, logPath := setupProject(t, "", "")
	t.Setenv("AZATH_TEST_FAIL_LAUNCH", "1")
	if err := cmdUp([]string{"demo"}); err == nil {
		t.Fatal("expected launch error")
	}
	launched := loadSession(t, launchStatePath)
	if !sessionIDPattern.MatchString(launched.CopilotSessionID) {
		t.Fatalf("expected saved UUID at launch, got %q", launched.CopilotSessionID)
	}
	if persisted := loadSession(t, statePath); launched != persisted {
		t.Fatalf("session state changed after launch failure: %+v", persisted)
	}
	if strings.Contains(readFile(t, logPath), "switch-client\n") {
		t.Fatal("attached after launch failure")
	}

	t.Setenv("AZATH_TEST_FAIL_LAUNCH", "")
	if err := cmdUp([]string{"demo"}); err != nil {
		t.Fatal(err)
	}
	if retried := loadSession(t, launchStatePath); retried.CopilotSessionID != launched.CopilotSessionID {
		t.Fatalf("retry changed session ID: %q", retried.CopilotSessionID)
	}
}

func TestResumeUsesSavedSessionID(t *testing.T) {
	statePath, _, logPath := setupProject(t, "", savedSessionID)
	if err := cmdResume([]string{"demo"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readFile(t, logPath), "\ncopilot --session-id="+savedSessionID+"\n") {
		t.Fatal("resume did not launch Copilot with the saved session ID")
	}
	if got := loadSession(t, statePath).CopilotSessionID; got != savedSessionID {
		t.Fatalf("resume changed session ID: %q", got)
	}
}

func setupProject(t *testing.T, projectConfig, previousID string) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.toml")
	launchStatePath := filepath.Join(dir, "launch-state.toml")
	logPath := filepath.Join(dir, "tmux.log")
	configPath := filepath.Join(dir, "config.toml")
	binDir := filepath.Join(dir, "bin")
	if err := os.Mkdir(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf("auto-discover = false\nstate-file = %q\n[project.demo]\npath = %q\n%s\n", statePath, dir, projectConfig)
	if err := os.WriteFile(configPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	const tmuxScript = `#!/bin/sh
set -eu
printf '%s\n' "$@" >> "$AZATH_TEST_TMUX_LOG"
case "$1" in
	has-session)
		if [ "${AZATH_TEST_BLOCK_SAVE:-}" = 1 ]; then
			mkdir -p "$AZATH_TEST_STATE"
		fi
		exit 1
		;;
	new-session)
		cp "$AZATH_TEST_STATE" "$AZATH_TEST_LAUNCH_STATE"
		if [ "${AZATH_TEST_FAIL_LAUNCH:-}" = 1 ]; then
			exit 1
		fi
		;;
	switch-client) ;;
	*) exit 1 ;;
esac
`
	if err := os.WriteFile(filepath.Join(binDir, "tmux"), []byte(tmuxScript), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", dir)
	t.Setenv("AZATH_CONFIG", configPath)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TMUX", "azath-test")
	t.Setenv("AZATH_TEST_STATE", statePath)
	t.Setenv("AZATH_TEST_LAUNCH_STATE", launchStatePath)
	t.Setenv("AZATH_TEST_TMUX_LOG", logPath)
	t.Setenv("AZATH_TEST_BLOCK_SAVE", "")
	t.Setenv("AZATH_TEST_FAIL_LAUNCH", "")
	if previousID != "" {
		st, err := state.Load(statePath)
		if err != nil {
			t.Fatal(err)
		}
		st.Set("demo", state.Session{CopilotSessionID: previousID})
		if err := st.Save(); err != nil {
			t.Fatal(err)
		}
	}
	return statePath, launchStatePath, logPath
}

func loadSession(t *testing.T, path string) state.Session {
	t.Helper()
	st, err := state.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	session, ok := st.Get("demo")
	if !ok {
		t.Fatalf("no demo session in %s", path)
	}
	return session
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
