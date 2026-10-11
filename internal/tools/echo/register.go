package echo

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Input struct {
	Text string `json:"text" jsonschema:"Text to echo back unchanged"`
}

type Output struct {
	Text string `json:"text" jsonschema:"The supplied text, echoed unchanged"`
}

func RegisterTool(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "echo_text",
		Title:       "Echo text",
		Description: "Return the supplied text unchanged.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input Input) (*mcp.CallToolResult, Output, error) {
		return nil, Output{Text: input.Text}, nil
	})
}
