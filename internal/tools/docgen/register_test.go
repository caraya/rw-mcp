package docgen

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

func TestDocumentationToolReturnsRootAndCommandPages(t *testing.T) {
	root := &cobra.Command{Use: "sample", Short: "Sample command-line application"}
	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Run:   func(*cobra.Command, []string) {},
	})

	server := mcp.NewServer(&mcp.Implementation{Name: "docgen-test", Version: "1.0.0"}, nil)
	RegisterTool(server, func() *cobra.Command { return root })

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
		Name: "cli_get_documentation",
		Arguments: map[string]any{
			"format": "markdown",
		},
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
	var output documentationOutput
	if err := json.Unmarshal(encoded, &output); err != nil {
		t.Fatalf("Unmarshal(StructuredContent) error = %v", err)
	}
	if output.Format != "markdown" {
		t.Fatalf("output format = %q, want markdown", output.Format)
	}
	pages := make(map[string]string, len(output.Documents))
	for _, page := range output.Documents {
		pages[page.Name] = page.Content
	}
	if pages["sample.md"] == "" || pages["sample_version.md"] == "" {
		t.Fatalf("documentation pages missing root or command: %v", pageNames(output.Documents))
	}
}

func pageNames(pages []documentationPage) []string {
	names := make([]string, 0, len(pages))
	for _, page := range pages {
		names = append(names, page.Name)
	}
	return names
}
