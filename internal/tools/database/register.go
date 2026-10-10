package database

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type noteIDInput struct {
	ID int64 `json:"id" jsonschema:"ID of the note to retrieve"`
}

type listNotesInput struct {
	Limit int `json:"limit,omitempty" jsonschema:"Maximum number of notes to return; defaults to 20 and is capped at 100"`
}

func RegisterTools(server *mcp.Server, store *Store) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "database_list_notes",
		Title:       "List local notes",
		Description: "List sample notes from the configured local SQLite database. The database is opened read-only.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input listNotesInput) (*mcp.CallToolResult, []Note, error) {
		notes, err := store.ListNotes(ctx, input.Limit)
		return nil, notes, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "database_get_note",
		Title:       "Get local note",
		Description: "Retrieve one note by its numeric ID from the configured local SQLite database.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input noteIDInput) (*mcp.CallToolResult, Note, error) {
		note, err := store.GetNote(ctx, input.ID)
		return nil, note, err
	})
}
