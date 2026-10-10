package server

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/carlos/rw-mcp/internal/tools/database"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestStdioSubprocessToolCallAndGracefulEOF(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "sample.db")
	if err := database.Initialize(databasePath); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	command := exec.Command("go", "run", "../../cmd/rw-mcp", "stdio", "--database", databasePath)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	client := mcp.NewClient(&mcp.Implementation{Name: "stdio-test-client", Version: "1.0.0"}, nil)
	clientSession, err := client.Connect(ctx, &mcp.CommandTransport{Command: command}, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v; stderr: %s", err, stderr.String())
	}

	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "database_get_note",
		Arguments: map[string]any{"id": 1},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v; stderr: %s", err, stderr.String())
	}
	if result.IsError || len(result.Content) == 0 {
		t.Fatalf("CallTool() result = %+v", result)
	}
	if err := clientSession.Close(); err != nil {
		t.Fatalf("ClientSession.Close() error = %v; stderr: %s", err, stderr.String())
	}
}
