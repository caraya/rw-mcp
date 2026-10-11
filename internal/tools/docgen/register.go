package docgen

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/carlos/rw-mcp/internal/clidocs"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

const maxDocumentationBytes = 256 << 10

type documentationInput struct {
	Format      string `json:"format,omitempty" jsonschema:"Output format: markdown, man, or rest. Defaults to markdown."`
	Frontmatter bool   `json:"frontmatter,omitempty" jsonschema:"Add YAML front matter to Markdown pages; ignored for other formats."`
}

type documentationPage struct {
	Name    string `json:"name" jsonschema:"Generated documentation filename"`
	Content string `json:"content" jsonschema:"Generated documentation content"`
}

type documentationOutput struct {
	Format    string              `json:"format" jsonschema:"Generated documentation format"`
	Documents []documentationPage `json:"documents" jsonschema:"One generated reference page per command"`
}

func RegisterTool(server *mcp.Server, rootCommandFactory func() *cobra.Command) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "cli_get_documentation",
		Title:       "Get CLI documentation",
		Description: "Generate reference pages from this server's Cobra command tree and return their contents. Generation uses a temporary directory and does not modify the workspace. Markdown is the recommended format for LLM use.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input documentationInput) (*mcp.CallToolResult, documentationOutput, error) {
		if err := ctx.Err(); err != nil {
			return nil, documentationOutput{}, err
		}
		format := input.Format
		if format == "" {
			format = "markdown"
		}
		outputDirectory, err := os.MkdirTemp("", "rw-mcp-cli-docs-")
		if err != nil {
			return nil, documentationOutput{}, fmt.Errorf("create temporary documentation directory: %w", err)
		}
		defer os.RemoveAll(outputDirectory)

		if err := clidocs.Generate(rootCommandFactory, outputDirectory, format, input.Frontmatter); err != nil {
			return nil, documentationOutput{}, err
		}
		pages, err := readDocumentation(outputDirectory)
		if err != nil {
			return nil, documentationOutput{}, err
		}
		return nil, documentationOutput{Format: format, Documents: pages}, nil
	})
}

func readDocumentation(directory string) ([]documentationPage, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("list generated documentation: %w", err)
	}
	pages := make([]documentationPage, 0, len(entries))
	remaining := int64(maxDocumentationBytes)
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		file, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("open generated documentation %q: %w", entry.Name(), err)
		}
		content, readErr := io.ReadAll(io.LimitReader(file, remaining+1))
		closeErr := file.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read generated documentation %q: %w", entry.Name(), readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close generated documentation %q: %w", entry.Name(), closeErr)
		}
		if int64(len(content)) > remaining {
			return nil, fmt.Errorf("generated documentation exceeds %d bytes", maxDocumentationBytes)
		}
		remaining -= int64(len(content))
		pages = append(pages, documentationPage{Name: entry.Name(), Content: string(content)})
	}
	return pages, nil
}
