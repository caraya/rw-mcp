# v1.0.0 MVP Release Plan

This plan defines the release gate for the first stable MVP of `rw-mcp`. The MVP is a local MCP server over stdio; it does not include remote hosting or OAuth.

## MVP Scope

The release provides:

- Typed MCP tools for echo, GitHub examples, U.S. weather forecasts, and narrowly scoped read-only SQLite access.
- Optional configured agent execution and a local Markdown catalog. `agent_invoke` uses the server-owned agent/skill mapping and does not receive this server's other tool schemas or handlers.
- A Cobra CLI, generated CLI documentation tool, setup guidance, and GoReleaser artifacts for the configured platforms.

Remote MCP transport and authentication, outbound MCP connections, and write-capable database operations are not MVP requirements. Track them in [backlog.md](backlog.md) and add them only when a concrete use case justifies the scope.

## Release Steps

1. **Confirm scope and configuration.** Review the README tool list, supported Go version, environment variables, database behavior, and agent trust boundary. Confirm that optional `RW_MCP_AGENT_SKILLS_SHA` checks the configured local checkout as documented; it does not clone or update that checkout.
2. **Review and settle changes.** Review all intended changes, including tests and user-facing documentation. Resolve or explicitly defer open defects. Start the release from a clean worktree and ensure no local-only files are required to build or run the server.
3. **Establish automated checks.** Configure CI to run `go test ./...`, `go vet ./...`, formatting validation, and `goreleaser check` on pull requests and the release branch. Keep the Go version aligned with the documented minimum/toolchain requirements.
4. **Run release validation.** Run the full tests, race tests for concurrency-sensitive packages, `go vet ./...`, `git diff --check`, and `goreleaser check`. Build the executable and configured release artifacts locally with `goreleaser build --clean`; inspect artifact names, checksums, and version output.
5. **Smoke-test the user workflow.** Initialize a temporary database, launch the stdio server through a representative MCP host/client, list tools, and call representative local tools. Verify optional tools appear only when configured, credentials are not required for the anonymous examples, and stdout contains only MCP protocol messages.
6. **Publish the release.** Merge the validated changes and create and push the `v1.0.0` tag. Create a [fine-grained personal access token](https://github.com/settings/personal-access-tokens/new) scoped to the target repository with the **Contents: Read and write** repository permission, then set it as `GITHUB_TOKEN`. Keep the token in the environment or secret store; do not commit or log it. Run `goreleaser release --clean`. Verify the release page contains all expected platform archives and checksums, then install and run an artifact on at least one supported platform.
7. **Close the release loop.** Record the published version and any known limitations. Move deferred work to [backlog.md](backlog.md); do not silently expand the v1.0.0 scope during release.

## Acceptance Criteria

- CI passes on the release commit, and the worktree used to tag is clean.
- The documented build, database initialization, stdio startup, and representative MCP calls work as written.
- All package tests and `go vet` pass; release configuration validates and local artifacts build successfully.
- Release artifacts and checksums are published for every target configured in `.goreleaser.yaml`.
- Known limitations are documented, especially stdio-only transport, lack of OAuth, and the fact that agent Markdown is trusted prompt input rather than an enforcement mechanism.
