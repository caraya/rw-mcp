package server

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/carlos/rw-mcp/internal/buildinfo"
	"github.com/carlos/rw-mcp/internal/tools/agent"
	"github.com/carlos/rw-mcp/internal/tools/database"
	"github.com/carlos/rw-mcp/internal/tools/echo"
	"github.com/carlos/rw-mcp/internal/tools/github"
	"github.com/carlos/rw-mcp/internal/tools/weather"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Config struct {
	DatabasePath          string
	GitHubToken           string
	GitHubAPIBaseURL      string
	NWSUserAgent          string
	OpenMeteoGeocodingURL string
	OpenMeteoAPIKey       string
	AgentCommand          string
	AgentArgs             []string
	AgentSkillsDir        string
	AgentTimeout          time.Duration
	AgentMaxOutput        int
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
			"It can also return generated command-line reference pages through cli_get_documentation, and can expose a configured local agent-skills catalog. " +
			"The server uses stdio and does not use OAuth.",
	})

	echo.RegisterTool(mcpServer)
	githubClient := github.NewClient(&http.Client{Timeout: 15 * time.Second}, config.GitHubToken, config.GitHubAPIBaseURL)
	github.RegisterTools(mcpServer, githubClient)
	weatherHTTPClient := &http.Client{Timeout: 15 * time.Second}
	nwsClient := weather.NewClient(weatherHTTPClient, "", config.NWSUserAgent)
	geocoder, err := weather.NewGeocoder(weatherHTTPClient, config.OpenMeteoGeocodingURL, config.OpenMeteoAPIKey)
	if err != nil {
		_ = store.Close()
		return nil, nil, err
	}
	weather.RegisterTool(mcpServer, nwsClient, geocoder)
	database.RegisterTools(mcpServer, store)

	var runner agent.Runner
	if config.AgentCommand != "" {
		commandRunner, err := agent.NewCommandRunner(config.AgentCommand, config.AgentArgs, config.AgentTimeout, config.AgentMaxOutput)
		if err != nil {
			_ = store.Close()
			return nil, nil, err
		}
		runner = commandRunner
		agent.RegisterTool(mcpServer, runner)
	}
	if config.AgentSkillsDir != "" {
		catalog, err := agent.LoadCatalog(config.AgentSkillsDir)
		if err != nil {
			_ = store.Close()
			return nil, nil, err
		}
		agent.RegisterCatalogTools(mcpServer, catalog, runner)
	}

	return mcpServer, store, nil
}
