package state

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

type Session struct {
	TmuxSession      string    `toml:"tmux-session"`
	CopilotSessionID string    `toml:"copilot-session-id,omitempty"`
	StartedAt        time.Time `toml:"started-at"`
	LastUsedAt       time.Time `toml:"last-used-at"`
}

type State struct {
	Sessions map[string]Session `toml:"session"`
	path     string
}

func Load(path string) (*State, error) {
	s := &State{Sessions: map[string]Session{}, path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	if _, err := toml.Decode(string(data), s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if s.Sessions == nil {
		s.Sessions = map[string]Session{}
	}
	return s, nil
}

func (s *State) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".state-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	enc := toml.NewEncoder(tmp)
	if err := enc.Encode(s); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.path)
}

func (s *State) Get(name string) (Session, bool) {
	v, ok := s.Sessions[name]
	return v, ok
}

func (s *State) Set(name string, sess Session) {
	s.Sessions[name] = sess
}

func (s *State) Delete(name string) {
	delete(s.Sessions, name)
}

// Prune removes session entries whose name is not in keep. Returns the names
// that were pruned. Caller is responsible for Save().
func (s *State) Prune(keep []string) []string {
	set := make(map[string]bool, len(keep))
	for _, k := range keep {
		set[k] = true
	}
	var removed []string
	for name := range s.Sessions {
		if !set[name] {
			removed = append(removed, name)
			delete(s.Sessions, name)
		}
	}
	return removed
}
