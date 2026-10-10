# MCP Server Guidance

- Use the official [`modelcontextprotocol/go-sdk`](https://github.com/modelcontextprotocol/go-sdk) for MCP protocol and transport behavior.
- Follow the [MCP `2026-07-28` specification](https://modelcontextprotocol.io/specification/2026-07-28), especially [stdio](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/stdio) and [tools](https://modelcontextprotocol.io/specification/2026-07-28/server/tools).
- Keep stdout reserved for SDK-managed MCP messages. Send logs and process errors to stderr.
- Add tools through the SDK's typed registration API. Keep credentials in server configuration/environment, never tool arguments.
- Local database tools must remain narrowly scoped, parameterized, and read-only.