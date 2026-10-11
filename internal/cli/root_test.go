package cli

import (
	"bytes"
	"os"
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

func TestDocsCommandGeneratesMarkdown(t *testing.T) {
	outputDirectory := filepath.Join(t.TempDir(), "cli-docs")
	command := NewRootCommand()
	command.SetArgs([]string{"docs", "--out", outputDirectory})
	command.SetOut(&bytes.Buffer{})
	command.SetErr(&bytes.Buffer{})
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDirectory, "rw-mcp.md")); err != nil {
		t.Fatalf("expected generated root Markdown file: %v", err)
	}
}

func TestDocsCommandRejectsUnknownFormat(t *testing.T) {
	command := NewRootCommand()
	command.SetArgs([]string{"docs", "--format", "xml", "--out", t.TempDir()})
	command.SetOut(&bytes.Buffer{})
	command.SetErr(&bytes.Buffer{})
	if err := command.Execute(); err == nil {
		t.Fatal("Execute() error = nil, want unsupported format error")
	}
}
