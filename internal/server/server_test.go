package server

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
		"echo_text",
		"github_get_public_repository",
		"github_get_authenticated_user",
		"nws_get_forecast",
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
	if found["agent_list_assets"] || found["agent_invoke"] {
		t.Error("ListTools() unexpectedly contains agent catalog tools without a configured skills directory")
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
	agentSkillsDirectory := filepath.Join(t.TempDir(), "agent-skills")
	agentPromptPath := filepath.Join(agentSkillsDirectory, "agents", "example", "agent.md")
	if err := os.MkdirAll(filepath.Dir(agentPromptPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(agentPromptPath, []byte("# Example Agent\nFollow the task."), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	skillPath := filepath.Join(agentSkillsDirectory, "skills", "go-core", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(skillPath, []byte("# Go Core\nUse idiomatic Go."), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	server, store, err := New(Config{
		DatabasePath:   databasePath,
		AgentCommand:   commandPath,
		AgentSkillsDir: agentSkillsDirectory,
	})
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
	for _, name := range []string{"agent_run", "agent_list_assets", "agent_invoke"} {
		if !found[name] {
			t.Errorf("ListTools() missing %q", name)
		}
	}
}

func TestConfiguredAgentSkillsSHARejectsMismatch(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "sample.db")
	if err := database.Initialize(databasePath); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	agentSkillsDirectory := t.TempDir()
	agentPromptPath := filepath.Join(agentSkillsDirectory, "agents", "example", "agent.md")
	if err := os.MkdirAll(filepath.Dir(agentPromptPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(agentPromptPath, []byte("# Example Agent\nFollow the task."), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	runGit(t, agentSkillsDirectory, "init", "--quiet")
	runGit(t, agentSkillsDirectory, "add", ".")
	runGit(t, agentSkillsDirectory, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--quiet", "-m", "initial")

	_, store, err := New(Config{
		DatabasePath:   databasePath,
		AgentSkillsDir: agentSkillsDirectory,
		AgentSkillsSHA: strings.Repeat("0", 40),
	})
	if err == nil {
		if store != nil {
			_ = store.Close()
		}
		t.Fatal("New() error = nil for a mismatched agent skills SHA")
	}
}

func runGit(t *testing.T, directory string, args ...string) {
	t.Helper()
	commandArgs := append([]string{"-C", directory}, args...)
	if output, err := exec.Command("git", commandArgs...).CombinedOutput(); err != nil {
		t.Fatalf("git %v error = %v, output = %s", args, err, output)
	}
}
