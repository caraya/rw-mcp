package github

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type repositoryInput struct {
	Owner      string `json:"owner" jsonschema:"GitHub repository owner"`
	Repository string `json:"repository" jsonschema:"GitHub repository name"`
}

func RegisterTools(server *mcp.Server, client *Client) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "github_get_public_repository",
		Title:       "Get public GitHub repository",
		Description: "Look up a public GitHub repository without sending an API token.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input repositoryInput) (*mcp.CallToolResult, Repository, error) {
		repository, err := client.GetRepository(ctx, input.Owner, input.Repository)
		return nil, repository, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "github_get_authenticated_user",
		Title:       "Get authenticated GitHub user",
		Description: "Return the GitHub user associated with GITHUB_TOKEN. Requires a token in the server process environment.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, User, error) {
		user, err := client.GetAuthenticatedUser(ctx)
		return nil, user, err
	})
}
