package cli

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/carlos/rw-mcp/internal/tools/database"
)

func TestDatabaseInitCommandCreatesReadableSampleDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.db")
	command := NewRootCommand()
	command.SetArgs([]string{"db", "init", "--path", path})
	command.SetOut(&bytes.Buffer{})
	command.SetErr(&bytes.Buffer{})
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	store, err := database.OpenReadOnly(path)
	if err != nil {
		t.Fatalf("OpenReadOnly() error = %v", err)
	}
	defer store.Close()
	notes, err := store.ListNotes(t.Context(), 10)
	if err != nil {
		t.Fatalf("ListNotes() error = %v", err)
	}
	if len(notes) != 2 {
		t.Fatalf("ListNotes() returned %d notes, want 2", len(notes))
	}
}
