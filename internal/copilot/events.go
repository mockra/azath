package copilot

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

// EventsPath returns the path to a session's events.jsonl.
func EventsPath(sessionDir, sessionID string) string {
	return filepath.Join(sessionDir, sessionID, "events.jsonl")
}

// EventsModTime returns the modtime of events.jsonl for a session. Returns a
// zero time when the file does not exist.
func EventsModTime(sessionDir, sessionID string) (time.Time, error) {
	if sessionDir == "" || sessionID == "" {
		return time.Time{}, nil
	}
	fi, err := os.Stat(EventsPath(sessionDir, sessionID))
	if err != nil {
		if os.IsNotExist(err) {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}
	return fi.ModTime(), nil
}

// LastUserMessage returns a one-line summary of the most recent user.message
// content in the session, truncated to maxRunes. Returns "" if no user message
// is present or the file is missing.
func LastUserMessage(sessionDir, sessionID string, maxRunes int) (string, error) {
	if sessionDir == "" || sessionID == "" {
		return "", nil
	}
	path := EventsPath(sessionDir, sessionID)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	defer f.Close()

	marker := []byte(`"type":"user.message"`)
	var last string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if !bytes.Contains(line, marker) {
			continue
		}
		var ev struct {
			Type string `json:"type"`
			Data struct {
				Content string `json:"content"`
			} `json:"data"`
		}
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		if ev.Type != "user.message" {
			continue
		}
		last = ev.Data.Content
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	return summarize(last, maxRunes), nil
}

func summarize(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	if maxRunes <= 0 || utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	return strings.TrimSpace(string(runes[:maxRunes-1])) + "…"
}
