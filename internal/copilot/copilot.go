package copilot

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DefaultSessionDir returns the conventional copilot session-state directory.
func DefaultSessionDir() string {
	return filepath.Join(os.Getenv("HOME"), ".copilot", "session-state")
}

func sessionStateDir() string { return DefaultSessionDir() }

// SnapshotSessionIDs returns the set of existing copilot session IDs (UUID
// directory names) in the default session-state dir.
func SnapshotSessionIDs() (map[string]bool, error) {
	return SnapshotSessionIDsDir(sessionStateDir())
}

// SnapshotSessionIDsDir snapshots UUID directory names in the given dir.
func SnapshotSessionIDsDir(dir string) (map[string]bool, error) {
	out := map[string]bool{}
	entries, err := os.ReadDir(dir)
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
	return NewestSessionSinceDir(before, sessionStateDir())
}

// NewestSessionSinceDir is the dir-explicit variant of NewestSessionSince.
func NewestSessionSinceDir(before map[string]bool, dir string) (string, error) {
	entries, err := os.ReadDir(dir)
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
