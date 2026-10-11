# Adding Integrations

This guide describes how to extend this MCP server with local tools, configured agents, outbound API integrations, and connections to other MCP servers. The project uses the official `github.com/modelcontextprotocol/go-sdk` v1.8.0 and the MCP `2026-07-28` protocol.

## Add a Tool

For a tool that this server should expose to MCP clients, put its implementation in its own package under `internal/tools/<name>/`. This is the default location for server-owned tools; keep tool implementation out of `cmd/`. Go's `internal/` visibility rule prevents other modules from importing these packages, so put a library intended for reuse outside this server in a non-`internal/` package or a separate module. Keep API or domain logic separate from MCP registration when that makes the package easier to test. Follow the existing file layout where useful: `client.go` for an outbound service client, `register.go` for MCP registration, and focused `*_test.go` files beside the code they cover. Small tools do not need empty or boilerplate files just to match that layout. See [internal/tools/github](../internal/tools/github/), [internal/tools/database](../internal/tools/database/), and [internal/tools/weather](../internal/tools/weather/).

For every tool intended to be callable over MCP, add package-level registration with the SDK's typed `mcp.AddTool` API and wire it into `server.New`. Use a `RegisterTool` or `RegisterTools` function, depending on whether the package exposes one or several tools. Construct dependencies in the server from trusted configuration and pass them to registration; do not let tool arguments choose credentials, arbitrary commands, or unrestricted service endpoints.

The typed API infers input and output schemas from the handler's Go types, validates incoming arguments, returns typed structured output, and converts handler errors into tool execution errors. JSON struct tags define field names, and `jsonschema` tags provide descriptions and constraints. There is no tool-source generator or scaffolding command in this repository: write the Go input/output types and typed registration, and the SDK builds the MCP schemas at registration time. The live echo tool in [internal/tools/echo/register.go](../internal/tools/echo/register.go) demonstrates this pattern:

```go
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
```

Wire it into [internal/server/server.go](internal/server/server.go):

```go
echo.RegisterTool(mcpServer)
```

Keep names unique, descriptions specific, and registration order deterministic. Add the registration whenever a new tool should be visible to clients; a helper package that is not an MCP tool does not need registration.

Add tests with the tool. At minimum, test the behavior that matters at the tool's boundary. For outbound APIs, use `httptest` to cover request construction, response decoding, and failure handling. For tool registration, add or extend a server test that connects over the SDK's in-memory transport and checks `ListTools` for the tool name; call it as well when input/output behavior or structured results need verification. Use existing focused tests as patterns: [internal/tools/weather/register_test.go](../internal/tools/weather/register_test.go) exercises a registered tool, and [internal/server/server_test.go](../internal/server/server_test.go) checks discovery. Tool annotations such as `ReadOnlyHint` are client-facing hints, not authorization checks. Enforce access controls in the handler and its dependencies.

For a new exposed tool, the usual sequence is:

1. Create `internal/tools/<name>/` and define typed input/output values with JSON and `jsonschema` tags.
2. Implement the behavior, then add `RegisterTool(s)` using `mcp.AddTool`.
3. Construct trusted dependencies and register the tool from `internal/server/server.go`.
4. Add focused behavior tests and verify the tool is discoverable through the server.
5. Run `go test ./internal/tools/<name> ./internal/server`, then `go test ./...` and `go vet ./...` before handoff.

### Example: Echo Tool

The `echo_text` tool is the smallest complete example in this repository. It has no remote service, credentials, configuration, or domain-specific client, so it needs only a registration file and a package test. The MCP schema is inferred from the Go input/output types; there is no generated schema or separate code-generation step.

| File | Responsibility |
| --- | --- |
| [internal/tools/echo/register.go](../internal/tools/echo/register.go) | Defines the typed `Input` and `Output`, their JSON/schema descriptions, the `echo_text` metadata, and `RegisterTool`. The handler returns the input text unchanged. |
| [internal/tools/echo/register_test.go](../internal/tools/echo/register_test.go) | Connects an in-memory MCP client and server, calls `echo_text`, and checks the structured output. This verifies the tool contract, not just a private helper. |
| [internal/server/server.go](../internal/server/server.go) | Imports the package and calls `echo.RegisterTool(mcpServer)` so the tool is exposed by the running server. This tool has no dependency to construct. |
| [internal/server/server_test.go](../internal/server/server_test.go) | Checks that `echo_text` appears in `ListTools` for a default server. This catches a missing server registration even when the tool package's own test passes. |
| [README.md](../README.md) | Lists `echo_text` in the server's user-facing tool catalog. |

Call it with a text field:

```json
{
  "text": "Hello from MCP"
}
```

The structured result is:

```json
{
  "text": "Hello from MCP"
}
```

To verify the example, run the package behavior test and server discovery test, then the repository-wide checks:

```sh
go test ./internal/tools/echo
go test ./internal/server -run TestNewRegistersDefaultToolsWithoutCredentials
go test ./...
go vet ./...
```

## Add an Outbound API Tool

An outbound API tool is an MCP tool in this server that makes an HTTP request to another service. It is different from connecting to another MCP server (see below).

Use the GitHub example as a reference: [internal/tools/github/client.go](internal/tools/github/client.go) owns HTTP behavior, while `register.go` exposes typed MCP handlers. When adding an API integration:

- Use `http.NewRequestWithContext` and a client with a finite timeout.
- Validate path segments and other user-provided values before building requests.
- Bound response reads, check status codes, decode only the fields the tool needs, and return actionable but non-sensitive errors.
- Keep API tokens in server configuration or environment variables, never in tool arguments. Do not log authorization headers or token values.
- Use `httptest` to verify the HTTP method, path, headers, response handling, missing credentials, and failure cases.

Update the server configuration and [internal/server/server.go](internal/server/server.go) to construct the API client and register its tools. A credential may be optional for some endpoints: keep anonymous tools usable when the credentialed example is not configured.

### Example: NWS Forecast

The built-in `nws_get_forecast` tool is a complete example of an anonymous remote API integration. It accepts either latitude/longitude or a U.S. city/postal-code `location`. For a place name, it calls Open-Meteo's Geocoding API, then calls `GET https://api.weather.gov/points/{latitude},{longitude}`, reads the `properties.forecast` URL, and fetches the forecast periods. NWS remains U.S.-only. Its requests send `Accept: application/geo+json` and a descriptive `User-Agent`; no NWS API key or OAuth flow is needed. The client limits response size, uses request contexts, and only follows forecast URLs on the NWS origin.

For example, call the tool with:

```json
{
  "latitude": 39.7456,
  "longitude": -97.0892
}
```

The structured result contains the nearby city/state and forecast periods with names, temperatures, short summaries, wind speeds, and directions. NWS forecast coverage is limited to U.S. locations. NWS requires a `User-Agent` identifying the application; this server defaults to `rw-mcp/0.1 (https://github.com/caraya/rw-mcp)`. Set `NWS_USER_AGENT` to a descriptive value with your preferred contact URL or email when deploying your own instance. This value is not a secret.

For example, resolve a place name:

```json
{
  "location": "Topeka, Kansas"
}
```

The location string is sent to Open-Meteo, so do not include confidential location data. Open-Meteo may return multiple U.S. matches; the tool rejects ambiguous exact matches and asks for a state or postal code rather than silently selecting one. The default public geocoding endpoint is keyless, but its free API is for non-commercial use only and is limited to fewer than 10,000 requests per day, 5,000 per hour, and 600 per minute. For commercial use, configure the endpoint and API key provided by an Open-Meteo paid plan using `OPEN_METEO_GEOCODING_URL` and `OPEN_METEO_API_KEY`. The server rejects a key without an explicit endpoint so credentials are not sent to the public endpoint. Keys remain in server environment configuration. When a place name is used, the structured forecast includes attribution: Open-Meteo location data based on GeoNames, CC BY 4.0. See Open-Meteo's [Geocoding API documentation](https://open-meteo.com/en/docs/geocoding-api) and [terms](https://open-meteo.com/en/terms).

The client, typed tool registration, server wiring, and HTTP test are in [internal/tools/weather/client.go](internal/tools/weather/client.go), [internal/tools/weather/register.go](internal/tools/weather/register.go), [internal/server/server.go](internal/server/server.go), and [internal/tools/weather/client_test.go](internal/tools/weather/client_test.go).

## Add an Agent

The current agent integration is a standalone executable adapter in [internal/tools/agent/runner.go](internal/tools/agent/runner.go). It implements the small `Runner` interface and is exposed by [internal/tools/agent/register.go](internal/tools/agent/register.go). The server registers it only when `RW_MCP_AGENT_COMMAND` is configured.

To integrate another agent:

1. Implement `Runner` with `Run(context.Context, string) (string, error)`, or add a separate adapter if the agent needs a richer input/output contract.
2. Construct it from trusted server configuration in `internal/server/server.go` and register the corresponding MCP tool.
3. Add tests for cancellation, errors, output limits, and any external API boundary.

For subprocess adapters, use `exec.CommandContext` with an executable and fixed arguments configured by the operator. Pass model-controlled text only as data; do not build shell command strings or let tool arguments choose an executable, URL, or arbitrary command flags. Preserve the existing timeout and output bounds unless the integration has a justified alternative.

The current `agent_run` and `agent_invoke` tools are prompt-in/text-out adapters. `agent_invoke` adds selected local agent/skill/resource Markdown to the prompt, but it does not forward this server's registered MCP tools to the child process. This is an intentional isolation boundary: agent and skill prompts can select only Markdown assets from the configured checkout, while the MCP host calls `rw-mcp` tools separately. Do not add a bridge from agent execution to this server's tool handlers. The configured executable still has whatever operating-system permissions and separately configured integrations its operator gives it; this MCP integration does not sandbox that process.

### Load Agents and Skills from a Local Checkout

Agent and skill files are Markdown instructions, not Go packages or executable plugins. To make the files in [`caraya/agent-skills`](https://github.com/caraya/agent-skills) available to this MCP server, check out that repository locally and set `RW_MCP_AGENT_SKILLS_DIR` to its root. The server scans Markdown files at startup and does not download content from GitHub during tool calls.

Supported primary prompt layouts include `agents/<agent>/agent.md` (and flat `agents/<agent>.md`) and `skills/<skill>/SKILL.md`. Other Markdown files in the checkout, including `references/` content, are exposed as resources. Files are bounded when loading: at most 512 Markdown files, 1 MiB per file, and 16 MiB total. The operator-selected directory is trusted input; its content becomes part of prompts sent to the configured agent executable.

The MCP surface has two additional tools when this directory is configured:

- `agent_list_assets` lists stable IDs, titles, and checkout-relative paths for available agents, skills, and resources. It is available without an agent executable.
- `agent_invoke` is available when both `RW_MCP_AGENT_SKILLS_DIR` and `RW_MCP_AGENT_COMMAND` are set. It requires a task and a policy-supported agent, plus optional resources from the catalog. Callers cannot select skills: `content-editor` always receives `style-lookup` and `code-example-validation`; `polyglot-engineering` and `polyglot-performance` require one `language` (`typescript`, `go`, `rust`, or `python`) and receive exactly that language's core, debugging, formatting, and performance skills. `agent_list_assets` reports the supported profiles available in the checkout. Unsupported agents and languages, or profiles with missing skills, are rejected. The tool composes the policy-selected Markdown instructions and resources with the task, then sends the result as one argument to the configured executable. It never accepts a command, filesystem path, or URL from the MCP caller.

The existing `agent_run` remains available with `RW_MCP_AGENT_COMMAND` alone for callers that want to provide a free-form prompt. All execution uses the configured executable's permissions; the Markdown does not sandbox or constrain that executable. Review the agent command and local skill checkout before enabling invocation.

Example server environment:

```sh
RW_MCP_AGENT_COMMAND=/absolute/path/to/agent
RW_MCP_AGENT_ARGS='["--print"]'
RW_MCP_AGENT_SKILLS_DIR=/absolute/path/to/agent-skills
```

The implementation is in [internal/tools/agent/catalog.go](../internal/tools/agent/catalog.go) and [internal/tools/agent/register.go](../internal/tools/agent/register.go); startup wiring is in [internal/server/server.go](../internal/server/server.go) and [internal/cli/root.go](../internal/cli/root.go). The catalog is loaded at startup, so restart the server after changing checkout contents.

## Connect to Another MCP Server

Yes. The process can serve its host as an MCP server and also create an `mcp.Client` connection to another MCP server. These are separate MCP connections with opposite roles. This project currently does not create outbound MCP client sessions; it only calls regular HTTP APIs and exposes its own tools.

The SDK supports this directly. For a local MCP subprocess, use `mcp.CommandTransport`; for a remote Streamable HTTP MCP endpoint, use `mcp.StreamableClientTransport`. In either case, create an `mcp.Client`, call `Client.Connect`, and use the returned `ClientSession` methods such as `CallTool`. The SDK's getting-started example shows the subprocess transport pattern: [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk).

A separate outbound MCP client integration could connect this server to a configured remote MCP server for an explicitly designed server-side workflow. It must not expose remote or local MCP tools to `agent_invoke`; preserve the agent/skill isolation boundary above. If such an integration is added for other server workflows, treat it as privileged:

- Keep remote endpoints and subprocess commands in operator configuration, not tool arguments. In particular, do not accept arbitrary URLs or commands from model-controlled input.
- Configure authentication on the outbound client side and keep credentials out of schemas and logs. This server's stdio transport itself does not require OAuth.
- Expose an explicit allowlist of remote tool names, prefix names to avoid collisions, and define how the bridge handles remote errors, timeouts, cancellation, and session shutdown.
- Make the user-visible descriptions clear that a call crosses into another server and may access that server's data or capabilities.

## Test and Verify

Follow the existing tests as patterns:

- HTTP API behavior: `httptest` in [internal/tools/github/client_test.go](internal/tools/github/client_test.go).
- Local data behavior: a temporary SQLite file in [internal/tools/database/store_test.go](internal/tools/database/store_test.go).
- Tool discovery and registration: SDK in-memory transports in [internal/server/server_test.go](internal/server/server_test.go).
- Stdio interoperability: a real SDK command transport in [internal/server/stdio_test.go](internal/server/stdio_test.go).

Run the full checks after adding an integration:

```sh
gofmt -w ./cmd ./internal
go test ./...
go vet ./...
```
