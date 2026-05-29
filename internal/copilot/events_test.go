package copilot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeEvents(t *testing.T, dir, id string, lines []string) string {
	t.Helper()
	sessionDir := filepath.Join(dir, id)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sessionDir, "events.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLastUserMessage(t *testing.T) {
	dir := t.TempDir()
	id := "550e8400-e29b-41d4-a716-446655440000"
	writeEvents(t, dir, id, []string{
		`{"type":"session.start","data":{}}`,
		`{"type":"user.message","data":{"content":"first message"}}`,
		`{"type":"assistant.message","data":{"content":"ack"}}`,
		`{"type":"user.message","data":{"content":"please refactor the dashboard module\nand add tests"}}`,
		`{"type":"assistant.turn_end","data":{}}`,
	})

	got, err := LastUserMessage(dir, id, 80)
	if err != nil {
		t.Fatalf("LastUserMessage: %v", err)
	}
	want := "please refactor the dashboard module"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLastUserMessageTruncates(t *testing.T) {
	dir := t.TempDir()
	id := "550e8400-e29b-41d4-a716-446655440001"
	writeEvents(t, dir, id, []string{
		`{"type":"user.message","data":{"content":"abcdefghijklmnopqrstuvwxyz"}}`,
	})
	got, err := LastUserMessage(dir, id, 10)
	if err != nil {
		t.Fatalf("LastUserMessage: %v", err)
	}
	if got != "abcdefghi…" {
		t.Errorf("got %q, want truncated", got)
	}
}

func TestLastUserMessageMissing(t *testing.T) {
	dir := t.TempDir()
	got, err := LastUserMessage(dir, "550e8400-e29b-41d4-a716-446655440002", 80)
	if err != nil {
		t.Fatalf("LastUserMessage: %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestLastUserMessageEmptyArgs(t *testing.T) {
	if got, _ := LastUserMessage("", "", 80); got != "" {
		t.Errorf("got %q, want empty", got)
	}
	if got, _ := LastUserMessage(t.TempDir(), "", 80); got != "" {
		t.Errorf("got %q, want empty for empty id", got)
	}
}

func TestEventsModTime(t *testing.T) {
	dir := t.TempDir()
	id := "550e8400-e29b-41d4-a716-446655440003"
	path := writeEvents(t, dir, id, []string{`{"type":"session.start","data":{}}`})

	when := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}

	got, err := EventsModTime(dir, id)
	if err != nil {
		t.Fatalf("EventsModTime: %v", err)
	}
	if !got.Equal(when) {
		t.Errorf("got %v, want %v", got, when)
	}
}

func TestEventsModTimeMissing(t *testing.T) {
	got, err := EventsModTime(t.TempDir(), "550e8400-e29b-41d4-a716-446655440004")
	if err != nil {
		t.Fatalf("EventsModTime: %v", err)
	}
	if !got.IsZero() {
		t.Errorf("got %v, want zero", got)
	}
}
