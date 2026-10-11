# MCP Server Backlog

This is a candidate backlog, not a commitment to implement every item. Prioritize based on deployment needs, threat model, and the current MCP specification. The v1.0.0 MVP remains a local stdio server; OAuth is not needed for that transport.

## Transport and Authentication

- [ ] Decide whether remote clients need to connect directly to this server. Keep stdio as the default unless there is a concrete remote-hosting use case.
- [ ] If remote access is needed, add an HTTP transport using the official MCP Go SDK and the current MCP transport specification. Define TLS termination, allowed origins/hosts, session lifecycle, request limits, and graceful shutdown.
- [ ] Add MCP client authentication at the transport/server boundary. If the selected remote transport and deployment require OAuth, implement it once for the protected MCP endpoint, not separately in each tool. Follow the current MCP authorization specification and test unauthenticated, authenticated, and insufficient-scope requests.
- [ ] Keep MCP-client authentication separate from credentials used by integrations. For example, `GITHUB_TOKEN` authorizes this server to call GitHub; it is not a credential for connecting an MCP client to this server. Continue to keep upstream credentials in trusted server configuration, never in tool arguments.

## MCP Capabilities and Integrations

- [ ] Preserve the agent/skill isolation boundary: `agent_invoke` may receive only the selected local catalog Markdown and task, never schemas or handlers for tools registered by `rw-mcp`. MCP hosts may call those tools separately.
- [ ] Evaluate whether stable, read-only data should be exposed as MCP resources in addition to tools. Define resource URIs, access boundaries, pagination, and update behavior before adding them.
- [ ] Evaluate prompt templates for recurring workflows. Add them only where a reusable prompt is more appropriate than a tool or documentation.
- [ ] Add an outbound MCP client integration for configured remote servers if there is a concrete need. Use operator-configured endpoints and credentials, an explicit allowlist of remote tools, and bounded timeouts/output. Do not accept arbitrary URLs, commands, or remote tool names from model-controlled input.
- [ ] Review each new tool's input/output schema, examples, errors, annotations, and side effects. Require explicit authorization for any future write-capable operations; annotations alone are not access control.
- [ ] Add integrations only with a clear upstream API policy, attribution requirements, credential model, and tests using local HTTP test servers where practical.

## Security and Reliability

- [ ] Document the trust boundary for model-provided input, configured agents, local database access, remote APIs, and any future HTTP transport.
- [ ] Keep network calls bounded by context cancellation, timeouts, response-size limits, and actionable non-sensitive errors. Add tests for these limits when integrations are introduced.
- [ ] Define per-tool or per-user rate limits if the server is exposed to multiple clients or usage grows. Avoid generic throttling before there is a deployment requirement and a clear identity key.
- [ ] Review logging for secret and sensitive-data leakage. Keep protocol output on stdout for stdio transport and logs on stderr.
- [ ] Test graceful shutdown and cancellation for each transport and any long-running tool or subprocess.

## Release and Maintenance

- [ ] Regenerate and review CLI reference docs when Cobra command metadata or flags change; keep command descriptions, examples, and groups useful to both people and LLMs.
- [ ] Review the MCP SDK and specification versions periodically. Update together with transport, authentication, tool-schema, and interoperability tests as needed.
- [ ] Add an interoperability smoke test against a representative MCP host/client when transport or protocol-facing behavior changes.
