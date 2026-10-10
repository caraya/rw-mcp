package database

import (
	"context"
	"path/filepath"
	"testing"
)

func TestInitializedDatabaseSupportsReadOnlyNoteTools(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.db")
	if err := Initialize(path); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	store, err := OpenReadOnly(path)
	if err != nil {
		t.Fatalf("OpenReadOnly() error = %v", err)
	}
	defer store.Close()

	note, err := store.GetNote(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetNote() error = %v", err)
	}
	if note.Title != "Welcome" {
		t.Fatalf("GetNote() = %+v, want Welcome note", note)
	}

	if _, err := store.db.ExecContext(context.Background(), "INSERT INTO notes(title, body) VALUES ('blocked', 'blocked')"); err == nil {
		t.Fatal("write to read-only database succeeded")
	}
}
