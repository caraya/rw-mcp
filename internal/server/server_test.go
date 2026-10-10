package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/carlos/rw-mcp/internal/tools/database"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestNewRegistersDefaultToolsWithoutCredentials(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "sample.db")
	if err := database.Initialize(databasePath); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	server, store, err := New(Config{DatabasePath: databasePath})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect() error = %v", err)
	}
	defer serverSession.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	defer clientSession.Close()

	result, err := clientSession.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	found := make(map[string]bool)
	for _, tool := range result.Tools {
		found[tool.Name] = true
	}
	for _, name := range []string{
		"github_get_public_repository",
		"github_get_authenticated_user",
		"database_list_notes",
		"database_get_note",
	} {
		if !found[name] {
			t.Errorf("ListTools() missing %q", name)
		}
	}
	if found["agent_run"] {
		t.Error("ListTools() unexpectedly contains unconfigured agent tool")
	}
}

func TestConfiguredAgentToolIsRegistered(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "sample.db")
	if err := database.Initialize(databasePath); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	commandPath := filepath.Join(t.TempDir(), "agent")
	if err := os.WriteFile(commandPath, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	server, store, err := New(Config{DatabasePath: databasePath, AgentCommand: commandPath})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect() error = %v", err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	defer clientSession.Close()
	result, err := clientSession.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	for _, tool := range result.Tools {
		if tool.Name == "agent_run" {
			return
		}
	}
	t.Fatal("ListTools() missing configured agent tool")
}
