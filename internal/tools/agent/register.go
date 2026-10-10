package agent

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type promptInput struct {
	Prompt string `json:"prompt" jsonschema:"Instructions to send to the configured agent"`
}

type promptOutput struct {
	Response string `json:"response" jsonschema:"Text returned by the configured agent"`
}

func RegisterTool(server *mcp.Server, runner Runner) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "agent_run",
		Title:       "Run configured agent",
		Description: "Send a prompt to the agent executable configured for this server. The executable and its arguments are fixed at server startup.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input promptInput) (*mcp.CallToolResult, promptOutput, error) {
		response, err := runner.Run(ctx, input.Prompt)
		return nil, promptOutput{Response: response}, err
	})
}
