package echo

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestEchoToolReturnsText(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "echo-test", Version: "1.0.0"}, nil)
	RegisterTool(server)

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

	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "echo_text",
		Arguments: map[string]any{"text": "Hello from MCP"},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool() returned tool error: %v", result.GetError())
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("Marshal(StructuredContent) error = %v", err)
	}
	var output struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(encoded, &output); err != nil {
		t.Fatalf("Unmarshal(StructuredContent) error = %v", err)
	}
	if output.Text != "Hello from MCP" {
		t.Fatalf("echoed text = %q, want %q", output.Text, "Hello from MCP")
	}
}
