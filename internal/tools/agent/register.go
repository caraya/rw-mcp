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

type invokeInput struct {
	Task      string   `json:"task" jsonschema:"Task to send to the selected agent profile"`
	Agent     string   `json:"agent" jsonschema:"Agent ID with a configured skill policy from agent_list_assets"`
	Language  string   `json:"language,omitempty" jsonschema:"Required for polyglot-engineering and polyglot-performance; choose go, python, rust, or typescript"`
	Resources []string `json:"resources,omitempty" jsonschema:"Resource IDs from agent_list_assets to include"`
}

type catalogListInput struct{}

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

func RegisterCatalogTools(server *mcp.Server, catalog *Catalog, runner Runner) {
	if catalog == nil {
		return
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "agent_list_assets",
		Title:       "List available agents and skills",
		Description: "List locally configured agent prompts, skills, and Markdown resources by stable ID. The catalog is loaded from operator configuration, not from tool-provided paths.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ catalogListInput) (*mcp.CallToolResult, catalogOutput, error) {
		return nil, catalog.list(), nil
	})

	if runner == nil {
		return
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "agent_invoke",
		Title:       "Run a policy-backed agent",
		Description: "Compose instructions using the selected agent's fixed skill policy and optional local resources, then send the task to the configured agent executable. Polyglot agents require an explicit language. The executable runs with the server process's permissions; use only trusted configuration and suitable tasks.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input invokeInput) (*mcp.CallToolResult, promptOutput, error) {
		if err := ctx.Err(); err != nil {
			return nil, promptOutput{}, err
		}
		prompt, err := catalog.composePrompt(input.Task, input.Agent, input.Language, input.Resources)
		if err != nil {
			return nil, promptOutput{}, err
		}
		response, err := runner.Run(ctx, prompt)
		return nil, promptOutput{Response: response}, err
	})
}
