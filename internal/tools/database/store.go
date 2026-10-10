package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const maxListLimit = 100

type Note struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

type Store struct {
	db *sql.DB
}

func Initialize(path string) error {
	absolutePath, err := absoluteDatabasePath(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(absolutePath), 0o700); err != nil {
		return fmt.Errorf("create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", absolutePath)
	if err != nil {
		return fmt.Errorf("open SQLite database: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("connect to SQLite database: %w", err)
	}

	transaction, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin SQLite initialization: %w", err)
	}
	defer transaction.Rollback()
	if _, err := transaction.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS notes (
			id INTEGER PRIMARY KEY,
			title TEXT NOT NULL,
			body TEXT NOT NULL
		)`); err != nil {
		return fmt.Errorf("create sample notes table: %w", err)
	}
	for _, note := range []Note{
		{ID: 1, Title: "Welcome", Body: "This is a sample local SQLite database exposed through read-only MCP tools."},
		{ID: 2, Title: "Next steps", Body: "Replace this sample schema and tools with narrowly scoped access to your own data."},
	} {
		if _, err := transaction.ExecContext(ctx,
			"INSERT INTO notes(id, title, body) VALUES (?, ?, ?) ON CONFLICT(id) DO NOTHING",
			note.ID, note.Title, note.Body,
		); err != nil {
			return fmt.Errorf("insert sample note %d: %w", note.ID, err)
		}
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit SQLite initialization: %w", err)
	}
	if err := os.Chmod(absolutePath, 0o600); err != nil {
		return fmt.Errorf("restrict SQLite database permissions: %w", err)
	}
	return nil
}

func OpenReadOnly(path string) (*Store, error) {
	absolutePath, err := absoluteDatabasePath(path)
	if err != nil {
		return nil, err
	}
	fileURI := (&url.URL{Scheme: "file", Path: filepath.ToSlash(absolutePath)}).String()
	parsedURI, err := url.Parse(fileURI)
	if err != nil {
		return nil, fmt.Errorf("build SQLite URI: %w", err)
	}
	query := parsedURI.Query()
	query.Set("mode", "ro")
	parsedURI.RawQuery = query.Encode()

	db, err := sql.Open("sqlite", parsedURI.String())
	if err != nil {
		return nil, fmt.Errorf("open SQLite database read-only: %w", err)
	}
	db.SetMaxOpenConns(4)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect to SQLite database read-only: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) GetNote(ctx context.Context, id int64) (Note, error) {
	if id <= 0 {
		return Note{}, errors.New("note ID must be greater than zero")
	}
	var note Note
	err := s.db.QueryRowContext(ctx,
		"SELECT id, title, body FROM notes WHERE id = ?",
		id,
	).Scan(&note.ID, &note.Title, &note.Body)
	if err != nil {
		return Note{}, fmt.Errorf("get note %d: %w", id, err)
	}
	return note, nil
}

func (s *Store) ListNotes(ctx context.Context, limit int) ([]Note, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > maxListLimit {
		return nil, fmt.Errorf("limit must not exceed %d", maxListLimit)
	}
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, title, body FROM notes ORDER BY id LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		var note Note
		if err := rows.Scan(&note.ID, &note.Title, &note.Body); err != nil {
			return nil, fmt.Errorf("scan note: %w", err)
		}
		notes = append(notes, note)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate notes: %w", err)
	}
	return notes, nil
}

func absoluteDatabasePath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("database path must not be empty")
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve database path: %w", err)
	}
	return absolutePath, nil
}
