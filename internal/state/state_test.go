package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMissingReturnsEmptyState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.toml")
	st, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st == nil {
		t.Fatal("expected state")
	}
	if len(st.Sessions) != 0 {
		t.Errorf("got %d sessions, want 0", len(st.Sessions))
	}
}

func TestSaveLoadRoundTripsSessions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.toml")
	now := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	st, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	st.Set("alpha", Session{
		TmuxSession:      "azath-alpha",
		CopilotSessionID: "550e8400-e29b-41d4-a716-446655440000",
		StartedAt:        now,
		LastUsedAt:       now.Add(time.Hour),
	})
	st.Set("beta", Session{
		TmuxSession: "azath-beta",
		StartedAt:   now.Add(2 * time.Hour),
		LastUsedAt:  now.Add(3 * time.Hour),
	})
	if err := st.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(got.Sessions) != 2 {
		t.Fatalf("got %d sessions, want 2", len(got.Sessions))
	}
	for name, want := range st.Sessions {
		gotSess, ok := got.Get(name)
		if !ok {
			t.Fatalf("missing session %q", name)
		}
		if gotSess != want {
			t.Errorf("session %q=%+v, want %+v", name, gotSess, want)
		}
	}
}

func TestGetUnknownReturnsFalse(t *testing.T) {
	st, err := Load(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if _, ok := st.Get("missing"); ok {
		t.Error("expected ok=false")
	}
}

func TestSetAndDelete(t *testing.T) {
	st, err := Load(filepath.Join(t.TempDir(), "state.toml"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	sess := Session{TmuxSession: "azath-alpha"}
	st.Set("alpha", sess)
	got, ok := st.Get("alpha")
	if !ok {
		t.Fatal("expected alpha session")
	}
	if got != sess {
		t.Errorf("got %+v, want %+v", got, sess)
	}
	st.Delete("alpha")
	if _, ok := st.Get("alpha"); ok {
		t.Error("expected alpha session to be deleted")
	}
}

func TestLoadInvalidTOMLReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.toml")
	if err := os.WriteFile(path, []byte("[session"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Error("expected parse error")
	}
}
