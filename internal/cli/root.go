package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/carlos/rw-mcp/internal/buildinfo"
	"github.com/carlos/rw-mcp/internal/server"
	"github.com/carlos/rw-mcp/internal/tools/database"
	"github.com/carlos/rw-mcp/internal/tools/docgen"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:               "rw-mcp",
		Short:             "Run the RW MCP server and manage its local tools",
		Long:              "Run the RW MCP server over stdio, initialize its sample local database, inspect the version, or generate reference documentation for the CLI.",
		Version:           buildinfo.Version,
		SilenceErrors:     true,
		SilenceUsage:      true,
		DisableAutoGenTag: true,
	}
	root.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	root.AddGroup(
		&cobra.Group{ID: "runtime", Title: "Runtime Commands"},
		&cobra.Group{ID: "database", Title: "Database Commands"},
		&cobra.Group{ID: "reference", Title: "Reference Commands"},
	)
	root.AddCommand(newStdioCommand(), newVersionCommand(), newDatabaseCommand(), newDocsCommand())
	return root
}

func newStdioCommand() *cobra.Command {
	databasePath := defaultDatabasePath()
	command := &cobra.Command{
		Use:   "stdio",
		Short: "Run the MCP server over standard input and output",
		Long:  "Run the MCP server over newline-delimited JSON-RPC on stdin/stdout. Logs are written to stderr. OAuth is not used.",
		Example: "  rw-mcp stdio --database data/sample.db\n" +
			"  RW_MCP_AGENT_COMMAND=/absolute/path/to/agent rw-mcp stdio",
		GroupID: "runtime",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			agentArgs, err := parseAgentArgs(os.Getenv("RW_MCP_AGENT_ARGS"))
			if err != nil {
				return err
			}
			mcpServer, store, err := server.New(server.Config{
				DatabasePath:          databasePath,
				GitHubToken:           os.Getenv("GITHUB_TOKEN"),
				NWSUserAgent:          os.Getenv("NWS_USER_AGENT"),
				OpenMeteoGeocodingURL: os.Getenv("OPEN_METEO_GEOCODING_URL"),
				OpenMeteoAPIKey:       os.Getenv("OPEN_METEO_API_KEY"),
				AgentCommand:          os.Getenv("RW_MCP_AGENT_COMMAND"),
				AgentArgs:             agentArgs,
				AgentSkillsDir:        os.Getenv("RW_MCP_AGENT_SKILLS_DIR"),
			})
			if err != nil {
				return err
			}
			defer store.Close()
			docgen.RegisterTool(mcpServer, NewRootCommand)

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			if err := mcpServer.Run(ctx, &mcp.StdioTransport{}); err != nil && !errors.Is(err, context.Canceled) {
				return err
			}
			return nil
		},
	}
	command.Flags().StringVar(&databasePath, "database", databasePath, "Path to the local SQLite database")
	return command
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "version",
		Short:   "Print the server version",
		Long:    "Print the version embedded in this rw-mcp build.",
		Example: "  rw-mcp version",
		GroupID: "runtime",
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "rw-mcp %s\n", buildinfo.Version)
		},
	}
}

func newDatabaseCommand() *cobra.Command {
	databaseCommand := &cobra.Command{
		Use:     "db",
		Short:   "Manage the sample local database",
		Long:    "Create and initialize the sample SQLite database used by the read-only database MCP tools.",
		GroupID: "database",
	}
	initPath := defaultDatabasePath()
	databaseCommand.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Create the sample SQLite database",
		Long:  "Create the sample database at the configured path. This command is intended for initial setup and may overwrite an existing database file.",
		Example: "  rw-mcp db init\n" +
			"  rw-mcp db init --path ./data/sample.db",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := database.Initialize(initPath); err != nil {
				return err
			}
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "Initialized sample database at %s\n", initPath)
			return err
		},
	})
	databaseCommand.Commands()[0].Flags().StringVar(&initPath, "path", initPath, "Path where the sample SQLite database will be created")
	return databaseCommand
}

func defaultDatabasePath() string {
	if path := os.Getenv("RW_MCP_DATABASE"); path != "" {
		return path
	}
	return "data/sample.db"
}

func parseAgentArgs(raw string) ([]string, error) {
	if raw == "" {
		return nil, nil
	}
	var args []string
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return nil, fmt.Errorf("RW_MCP_AGENT_ARGS must be a JSON array of strings: %w", err)
	}
	return args, nil
}
