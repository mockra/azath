package main

import (
	"time"

	"github.com/mockra/azath/internal/config"
	"github.com/mockra/azath/internal/copilot"
	"github.com/mockra/azath/internal/project"
	"github.com/mockra/azath/internal/state"
	"github.com/mockra/azath/internal/tmux"
)

type activity int

const (
	activityStopped activity = iota
	activityIdle
	activityWorking
)

// workingWindow is how recently events.jsonl must have changed for a
// project to be considered actively working.
const workingWindow = 5 * time.Second

// paneCaptureLines is the number of trailing pane lines hashed for the
// unread indicator and used as the "have I read this" baseline.
const paneCaptureLines = 50

// dashInfo bundles the runtime signals the dashboard surfaces per project.
type dashInfo struct {
	running     bool
	activity    activity
	unread      bool
	lastMessage string
	lastEvents  time.Time
}

// collectDashInfo gathers display signals for a project. It tolerates missing
// files (no copilot session yet, no events.jsonl) by returning zero values,
// so the picker stays responsive even when state is partial.
func collectDashInfo(cfg config.Config, st *state.State, p project.Project, sessionName string, running bool) dashInfo {
	info := dashInfo{running: running}
	if !running {
		info.activity = activityStopped
		return info
	}

	sess, _ := st.Get(p.Name)

	if cfg.CopilotSessionDir != "" && sess.CopilotSessionID != "" {
		if mod, err := copilot.EventsModTime(cfg.CopilotSessionDir, sess.CopilotSessionID); err == nil && !mod.IsZero() {
			info.lastEvents = mod
		}
		if msg, err := copilot.LastUserMessage(cfg.CopilotSessionDir, sess.CopilotSessionID, 80); err == nil {
			info.lastMessage = msg
		}
	}

	if !info.lastEvents.IsZero() && time.Since(info.lastEvents) < workingWindow {
		info.activity = activityWorking
	} else {
		info.activity = activityIdle
	}

	if hash := tmux.PaneHash(sessionName+":", paneCaptureLines); hash != "" {
		if sess.LastSeenPaneHash != "" && hash != sess.LastSeenPaneHash {
			info.unread = true
		}
	}

	return info
}

// markGlyph returns the colored status indicator for an activity.
func markGlyph(a activity) string {
	switch a {
	case activityWorking:
		return "\033[35m●\033[0m"
	case activityIdle:
		return "\033[32m●\033[0m"
	default:
		return "\033[2m○\033[0m"
	}
}

// unreadGlyph returns a bright bell when there is unread output, otherwise "".
func unreadGlyph(unread bool) string {
	if !unread {
		return ""
	}
	return "\033[1;33m*\033[0m"
}

// activityLabel returns the plain-text label used in `azath show`.
func activityLabel(a activity) string {
	switch a {
	case activityWorking:
		return "working"
	case activityIdle:
		return "idle"
	default:
		return "stopped"
	}
}
