# RW MCP Server

A small Go MCP server modeled on the command and package boundaries of the [GitHub MCP Server](https://github.com/github/github-mcp-server). It uses the official [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk) and targets the `2026-07-28` specification.

The server runs over stdio and does not implement OAuth. Its examples include an anonymous GitHub API call, an environment-token GitHub API call, an optional standalone agent CLI, and read-only local SQLite tools.

## Requirements

- Go 1.27.2 or newer to build from source

## Run

Build the server binary and initialize the sample database:

```sh
go build -o bin/rw-mcp ./cmd/rw-mcp
./bin/rw-mcp db init --path data/sample.db
```

The repository's VS Code config launches `bin/rw-mcp`. The server reads newline-delimited MCP messages from stdin and writes protocol messages to stdout. Logs and process errors go to stderr. Closing stdin shuts down the server.

Run `db init` once before connecting from VS Code.

### Why a Binary by Default

For stdio, an MCP host launches the server as a subprocess and exchanges newline-delimited JSON-RPC over stdin/stdout. MCP requires that process and transport behavior, not a particular packaging format. This project defaults to a prebuilt binary so Go is not needed at runtime.

For development, you can run directly from source with `go run ./cmd/rw-mcp stdio --database data/sample.db`; this compiles and launches a subprocess using the installed Go toolchain. Build binaries for the host's operating system and architecture.

## Tools

- `github_get_public_repository` calls `GET /repos/{owner}/{repo}` without sending a token.
- `github_get_authenticated_user` calls `GET /user` with `GITHUB_TOKEN`. It returns a tool error if the token is unset; the other tools remain available.
- `database_list_notes` and `database_get_note` use fixed, parameterized queries against the configured SQLite file. The server opens that file in SQLite read-only mode; it does not accept arbitrary SQL.
- `agent_run` is registered only when `RW_MCP_AGENT_COMMAND` is set. The executable and fixed arguments come from server configuration, never from tool input.

### Create a GitHub Token

To enable `github_get_authenticated_user`:

1. Open GitHub's [fine-grained personal access token settings](https://github.com/settings/personal-access-tokens/new) and sign in.
2. Choose a token name, expiration, and resource owner. Keep access limited to what you need; this example only calls `GET /user` and does not need repository write access.
3. Generate the token and copy it when GitHub displays it. Treat it like a password. GitHub's [token guide](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens) has the full steps and security guidance.

The tool returns public profile fields. If you use a classic personal access token and want the endpoint's private user fields, GitHub requires the `read:user` scope; those fields are not used by this example.

Provide the token to the server through the MCP host's secret or environment mechanism, not as a tool argument or in checked-in configuration. For VS Code, you can replace `.vscode/mcp.json` with this variant, which asks for the token using a password input instead of storing it in the file:

```json
{
  "inputs": [
    {
      "type": "promptString",
      "id": "github_token",
      "description": "GitHub personal access token",
      "password": true
    }
  ],
  "servers": {
    "rw-mcp": {
      "type": "stdio",
      "command": "${workspaceFolder}/bin/rw-mcp",
      "args": [
        "stdio",
        "--database",
        "${workspaceFolder}/data/sample.db"
      ],
      "cwd": "${workspaceFolder}",
      "env": {
        "GITHUB_TOKEN": "${input:github_token}"
      }
    }
  }
}
```

## Optional Agent CLI

Configure an installed agent executable in the MCP server process environment:

```sh
RW_MCP_AGENT_COMMAND=/absolute/path/to/agent
RW_MCP_AGENT_ARGS='["--print"]'
```

`RW_MCP_AGENT_ARGS` is a JSON array of fixed argv values. The prompt is appended as one final argument and is not interpreted by a shell. Agent calls have a 45-second default timeout and a 32 KiB output limit. The child process inherits the server's environment and permissions, so configure an agent command whose capabilities are appropriate for this MCP host.

## CLI

```sh
./bin/rw-mcp version
./bin/rw-mcp db init --path data/sample.db
./bin/rw-mcp stdio --database data/sample.db
```

`RW_MCP_DATABASE` sets the default database path. `--database` overrides it for `stdio`; `db init --path` selects where the sample file is created.

To use the CLI from source instead, prefix these commands with `go run ./cmd/rw-mcp`.

## Multi-Platform Builds

The `.goreleaser.yaml` configuration builds the server for macOS, Linux, and Windows on `amd64` and `arm64`. Builds disable CGO, package Unix binaries as `.tar.gz` and Windows binaries as `.zip`, and include a checksum file. The CLI and MCP server version are set from the release tag.

Install GoReleaser v2 and check the configuration:

```sh
go install github.com/goreleaser/goreleaser/v2@v2.18.0
goreleaser check
```

Build all configured targets locally without publishing:

```sh
goreleaser build --clean
```

Artifacts are written to `dist/`. To publish a GitHub release, create and push a semantic version tag, then run `goreleaser release --clean` with a `GITHUB_TOKEN` that can create releases in the repository. The local build command does not publish anything.

## Validate and Build

```sh
gofmt -w ./cmd ./internal
go test ./...
go vet ./...
go build -o bin/rw-mcp ./cmd/rw-mcp
```

The test suite uses local HTTP test servers, temporary SQLite files, fake agent executables, and in-memory MCP transports. No external API token or agent installation is required to run the tests.
