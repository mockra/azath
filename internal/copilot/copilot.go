package copilot

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func sessionStateDir() string {
	return filepath.Join(os.Getenv("HOME"), ".copilot", "session-state")
}

// NewSessionID generates a UUID for a new Copilot session.
func NewSessionID() (string, error) {
	var id [16]byte
	_, err := rand.Read(id[:])
	if err != nil {
		return "", fmt.Errorf("generate Copilot session ID: %w", err)
	}
	id[6] = (id[6] & 0x0f) | 0x40
	id[8] = (id[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:]), nil
}

func looksLikeUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
				return false
			}
		}
	}
	return true
}

// Status describes the live state of a Copilot CLI session.
type Status int

const (
	// StatusUnknown means the session id is empty or its state cannot be read.
	StatusUnknown Status = iota
	// StatusIdle means the session exists and is waiting for the user.
	StatusIdle
	// StatusWorking means the assistant or a tool is currently running.
	StatusWorking
)

// SessionStatus reports whether a copilot CLI session is currently working
// (generating, or running a tool) or idle (awaiting user input).
//
// It tails ~/.copilot/session-state/<sessionID>/events.jsonl and inspects the
// most recent event. assistant.turn_end / abort / session.task_complete signal
// idle; anything else (turn_start, tool.execution_start, user.message, etc.)
// signals working.
func SessionStatus(sessionID string) Status {
	if !looksLikeUUID(sessionID) {
		return StatusUnknown
	}
	path := filepath.Join(sessionStateDir(), sessionID, "events.jsonl")
	f, err := os.Open(path)
	if err != nil {
		return StatusUnknown
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil || info.Size() == 0 {
		return StatusUnknown
	}
	const window = 16 * 1024
	start := int64(0)
	if info.Size() > window {
		start = info.Size() - window
	}
	if _, err := f.Seek(start, 0); err != nil {
		return StatusUnknown
	}
	buf := make([]byte, info.Size()-start)
	if _, err := io.ReadFull(f, buf); err != nil {
		return StatusUnknown
	}
	lines := strings.Split(strings.TrimRight(string(buf), "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		switch eventType(lines[i]) {
		case "":
			continue
		case "assistant.turn_end", "abort", "session.shutdown", "session.task_complete":
			return StatusIdle
		case "assistant.turn_start", "tool.execution_start":
			return StatusWorking
		}
	}
	return StatusUnknown
}

// eventType extracts the "type" field from a JSONL event line cheaply, without
// fully unmarshalling.
func eventType(line string) string {
	const key = `"type":"`
	i := strings.Index(line, key)
	if i < 0 {
		return ""
	}
	rest := line[i+len(key):]
	j := strings.IndexByte(rest, '"')
	if j < 0 {
		return ""
	}
	return rest[:j]
}
