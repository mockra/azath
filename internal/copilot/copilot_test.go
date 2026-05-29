package copilot

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLooksLikeUUID(t *testing.T) {
	if !looksLikeUUID("550e8400-e29b-41d4-a716-446655440000") {
		t.Error("expected valid UUID")
	}
	cases := []string{
		"550e8400-e29b-41d4-a716-44665544000",
		"550e8400e29b41d4a716446655440000",
		"550e8400-e29b-41d4-a716-44665544000g",
	}
	for _, tc := range cases {
		if looksLikeUUID(tc) {
			t.Errorf("looksLikeUUID(%q)=true, want false", tc)
		}
	}
}

func TestSnapshotSessionIDsMissingDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	got, err := SnapshotSessionIDs()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d IDs, want 0", len(got))
	}
}

func TestSnapshotSessionIDsAndNewestSessionSince(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".copilot", "session-state")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	first := "550e8400-e29b-41d4-a716-446655440000"
	second := "650e8400-e29b-41d4-a716-446655440000"
	ignoredFile := "750e8400-e29b-41d4-a716-446655440000"
	ignoredDir := "not-a-session"
	if err := os.Mkdir(filepath.Join(dir, first), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, second), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ignoredFile), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, ignoredDir), 0o755); err != nil {
		t.Fatal(err)
	}

	snapshot, err := SnapshotSessionIDs()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(snapshot) != 2 {
		t.Fatalf("got %d IDs, want 2", len(snapshot))
	}
	if !snapshot[first] || !snapshot[second] {
		t.Errorf("snapshot=%v, want first and second", snapshot)
	}
	newest, err := NewestSessionSince(snapshot)
	if err != nil {
		t.Fatalf("newest before new dir: %v", err)
	}
	if newest != "" {
		t.Errorf("newest=%q, want empty", newest)
	}

	third := "750e8400-e29b-41d4-a716-446655440001"
	if err := os.Mkdir(filepath.Join(dir, third), 0o755); err != nil {
		t.Fatal(err)
	}
	when := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(dir, third), when, when); err != nil {
		t.Fatal(err)
	}

	newest, err = NewestSessionSince(snapshot)
	if err != nil {
		t.Fatalf("newest after new dir: %v", err)
	}
	if newest != third {
		t.Errorf("newest=%q, want %q", newest, third)
	}
}
