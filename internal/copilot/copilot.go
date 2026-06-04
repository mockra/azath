package copilot

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func sessionStateDir() string {
	return filepath.Join(os.Getenv("HOME"), ".copilot", "session-state")
}

// SnapshotSessionIDs returns the set of existing copilot session IDs (UUID
// directory names) before launching an agent. Compare with NewSessionIDs after
// the agent starts to discover the new session ID.
func SnapshotSessionIDs() (map[string]bool, error) {
	out := map[string]bool{}
	entries, err := os.ReadDir(sessionStateDir())
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if looksLikeUUID(name) {
			out[name] = true
		}
	}
	return out, nil
}

// NewestSessionSince returns the newest session-state UUID directory created
// since the snapshot was taken. Returns "" if none found.
func NewestSessionSince(before map[string]bool) (string, error) {
	entries, err := os.ReadDir(sessionStateDir())
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	type candidate struct {
		name    string
		modTime int64
	}
	var cands []candidate
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if !looksLikeUUID(name) {
			continue
		}
		if before[name] {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		cands = append(cands, candidate{name: name, modTime: info.ModTime().UnixNano()})
	}
	if len(cands) == 0 {
		return "", nil
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].modTime > cands[j].modTime })
	return cands[0].name, nil
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
