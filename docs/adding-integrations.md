# Adding Integrations

This guide describes how to extend this MCP server with local tools, configured agents, outbound API integrations, and connections to other MCP servers. The project uses the official `github.com/modelcontextprotocol/go-sdk` v1.8.0 and the MCP `2026-07-28` protocol.

## Add a Tool

Put a cohesive integration under `internal/tools/<name>/`. Keep its service/client logic separate from MCP registration where practical, and expose a `RegisterTools` function. The current examples are in [internal/tools/github/register.go](internal/tools/github/register.go) and [internal/tools/database/register.go](internal/tools/database/register.go).

The SDK's typed `mcp.AddTool` API infers input and output schemas from Go types, validates incoming arguments, returns typed structured output, and converts handler errors into tool execution errors. Use JSON and `jsonschema` tags to make the public schema clear:

```go
package example

import (
  "context"

  "github.com/modelcontextprotocol/go-sdk/mcp"
)

type EchoInput struct {
  Text string `json:"text" jsonschema:"Text to echo"`
}

type EchoOutput struct {
  Text string `json:"text" jsonschema:"Echoed text"`
}

func Register(server *mcp.Server) {
  mcp.AddTool(server, &mcp.Tool{
    Name:        "example_echo",
    Title:       "Echo text",
    Description: "Return the supplied text.",
  }, func(_ context.Context, _ *mcp.CallToolRequest, input EchoInput) (*mcp.CallToolResult, EchoOutput, error) {
    return nil, EchoOutput{Text: input.Text}, nil
  })
}
```

Register it from [internal/server/server.go](internal/server/server.go), after constructing its dependencies:

```go
example.Register(mcpServer)
```

Keep names unique, descriptions specific, and registration order deterministic. Tool annotations such as `ReadOnlyHint` are client-facing hints, not authorization checks. Enforce access controls in the handler and its dependencies.

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

## Connect to Another MCP Server

Yes. The process can serve its host as an MCP server and also create an `mcp.Client` connection to another MCP server. These are separate MCP connections with opposite roles. This project currently does not create outbound MCP client sessions; it only calls regular HTTP APIs and exposes its own tools.

The SDK supports this directly. For a local MCP subprocess, use `mcp.CommandTransport`; for a remote Streamable HTTP MCP endpoint, use `mcp.StreamableClientTransport`. In either case, create an `mcp.Client`, call `Client.Connect`, and use the returned `ClientSession` methods such as `CallTool`. The SDK's getting-started example shows the subprocess transport pattern: [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk).

A future bridge could connect to a configured remote MCP server and expose selected remote tools through this server. Treat that as a privileged integration:

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
