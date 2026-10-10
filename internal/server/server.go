package server

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/carlos/rw-mcp/internal/buildinfo"
	"github.com/carlos/rw-mcp/internal/tools/agent"
	"github.com/carlos/rw-mcp/internal/tools/database"
	"github.com/carlos/rw-mcp/internal/tools/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Config struct {
	DatabasePath     string
	GitHubToken      string
	GitHubAPIBaseURL string
	AgentCommand     string
	AgentArgs        []string
	AgentTimeout     time.Duration
	AgentMaxOutput   int
}

func New(config Config) (*mcp.Server, *database.Store, error) {
	store, err := database.OpenReadOnly(config.DatabasePath)
	if err != nil {
		return nil, nil, err
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "rw-mcp",
		Title:   "RW MCP Server",
		Version: buildinfo.Version,
	}, &mcp.ServerOptions{
		Logger: logger,
		Instructions: "Provides GitHub API examples, optional configured-agent execution, and read-only access to a local SQLite database. " +
			"The server uses stdio and does not use OAuth.",
	})

	githubClient := github.NewClient(&http.Client{Timeout: 15 * time.Second}, config.GitHubToken, config.GitHubAPIBaseURL)
	github.RegisterTools(mcpServer, githubClient)
	database.RegisterTools(mcpServer, store)

	if config.AgentCommand != "" {
		runner, err := agent.NewCommandRunner(config.AgentCommand, config.AgentArgs, config.AgentTimeout, config.AgentMaxOutput)
		if err != nil {
			_ = store.Close()
			return nil, nil, err
		}
		agent.RegisterTool(mcpServer, runner)
	}

	return mcpServer, store, nil
}
