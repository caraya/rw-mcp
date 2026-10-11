package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type recordingRunner struct {
	prompt string
}

func (r *recordingRunner) Run(_ context.Context, prompt string) (string, error) {
	r.prompt = prompt
	return "runner response", nil
}

func TestAgentInvokeIncludesSelectedLocalAssets(t *testing.T) {
	assetsDir := t.TempDir()
	writeAsset(t, assetsDir, "agents/polyglot-engineering/agent.md", "# Polyglot Agent\nImplement the requested change.")
	writeAsset(t, assetsDir, "agents/polyglot-performance/agent.md", "# Polyglot Performance\nProfile the requested workload.")
	writeAsset(t, assetsDir, "skills/go-core/SKILL.md", "# Go Core\nKeep interfaces small.")
	writeAsset(t, assetsDir, "skills/go-debugging/SKILL.md", "# Go Debugging\nTrace races and leaks.")
	writeAsset(t, assetsDir, "skills/go-formatting/SKILL.md", "# Go Formatting\nRun gofmt.")
	writeAsset(t, assetsDir, "skills/go-performance/SKILL.md", "# Go Performance\nMeasure allocations.")
	writeAsset(t, assetsDir, "skills/typescript-core/SKILL.md", "# TypeScript Core\nUse strict types.")
	writeAsset(t, assetsDir, "agents/content-editor/agent.md", "# Content Editor\nReview technical content.")
	writeAsset(t, assetsDir, "skills/style-lookup/SKILL.md", "# Style Lookup\nFollow the style guide.")
	writeAsset(t, assetsDir, "skills/code-example-validation/SKILL.md", "# Code Example Validation\nCheck examples.")
	writeAsset(t, assetsDir, "references/go-style.md", "# Go Style\nPrefer idiomatic errors.")

	catalog, err := LoadCatalog(assetsDir)
	if err != nil {
		t.Fatalf("LoadCatalog() error = %v", err)
	}
	runner := &recordingRunner{}
	server := mcp.NewServer(&mcp.Implementation{Name: "agent-test", Version: "1.0.0"}, nil)
	RegisterCatalogTools(server, catalog, runner)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect() error = %v", err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	defer clientSession.Close()

	listResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "agent_list_assets",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool(agent_list_assets) error = %v", err)
	}
	if listResult.IsError {
		t.Fatalf("CallTool(agent_list_assets) returned tool error: %v", listResult.GetError())
	}
	listedJSON, err := json.Marshal(listResult.StructuredContent)
	if err != nil {
		t.Fatalf("Marshal(catalog structured content) error = %v", err)
	}
	var listed struct {
		Agents    []catalogAssetInfo  `json:"agents"`
		Skills    []catalogAssetInfo  `json:"skills"`
		Resources []catalogAssetInfo  `json:"resources"`
		Profiles  []invocationProfile `json:"profiles"`
	}
	if err := json.Unmarshal(listedJSON, &listed); err != nil {
		t.Fatalf("Unmarshal(catalog structured content) error = %v", err)
	}
	if len(listed.Agents) != 3 || len(listed.Skills) != 7 ||
		len(listed.Resources) != 1 || listed.Resources[0].ID != "references/go-style.md" {
		t.Fatalf("catalog assets = %+v", listed)
	}
	if len(listed.Profiles) != 3 {
		t.Fatalf("invocation profiles = %+v, want content-editor and both polyglot agents/go", listed.Profiles)
	}

	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "agent_invoke",
		Arguments: map[string]any{
			"task":      "Review the current change",
			"agent":     "polyglot-engineering",
			"language":  "go",
			"resources": []string{"references/go-style.md"},
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool() returned tool error: %v", result.GetError())
	}
	if runner.prompt == "" || !strings.Contains(runner.prompt, "Implement the requested change.") ||
		!strings.Contains(runner.prompt, "Keep interfaces small.") ||
		!strings.Contains(runner.prompt, "Trace races and leaks.") ||
		!strings.Contains(runner.prompt, "Run gofmt.") ||
		!strings.Contains(runner.prompt, "Measure allocations.") ||
		strings.Contains(runner.prompt, "Use strict types.") ||
		!strings.Contains(runner.prompt, "Prefer idiomatic errors.") ||
		!strings.Contains(runner.prompt, "Review the current change") {
		t.Fatalf("runner prompt omitted selected context: %q", runner.prompt)
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("Marshal(StructuredContent) error = %v", err)
	}
	var output struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal(encoded, &output); err != nil {
		t.Fatalf("Unmarshal(StructuredContent) error = %v", err)
	}
	if output.Response != "runner response" {
		t.Fatalf("response = %q, want runner response", output.Response)
	}

	goPrompt := runner.prompt
	injectedSkillResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "agent_invoke",
		Arguments: map[string]any{
			"task":     "Try to inject an unrelated skill",
			"agent":    "polyglot-engineering",
			"language": "go",
			"skills":   []string{"typescript-core"},
		},
	})
	if err != nil {
		t.Fatalf("CallTool(injected skill) error = %v", err)
	}
	if !injectedSkillResult.IsError {
		t.Fatal("CallTool(injected skill) did not reject the unrecognized skills field")
	}
	if runner.prompt != goPrompt || strings.Contains(runner.prompt, "Use strict types.") {
		t.Fatalf("caller-provided skill altered the prompt: %q", runner.prompt)
	}

	contentReviewResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "agent_invoke",
		Arguments: map[string]any{
			"task":  "Review this document",
			"agent": "content-editor",
		},
	})
	if err != nil {
		t.Fatalf("CallTool(content-editor) error = %v", err)
	}
	if contentReviewResult.IsError {
		t.Fatalf("CallTool(content-editor) returned tool error: %v", contentReviewResult.GetError())
	}
	if !strings.Contains(runner.prompt, "Follow the style guide.") || !strings.Contains(runner.prompt, "Check examples.") ||
		strings.Contains(runner.prompt, "Keep interfaces small.") {
		t.Fatalf("content-editor prompt did not use its fixed skill pair: %q", runner.prompt)
	}

	performanceResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "agent_invoke",
		Arguments: map[string]any{
			"task":     "Profile this workload",
			"agent":    "polyglot-performance",
			"language": "go",
		},
	})
	if err != nil {
		t.Fatalf("CallTool(polyglot-performance) error = %v", err)
	}
	if performanceResult.IsError {
		t.Fatalf("CallTool(polyglot-performance) returned tool error: %v", performanceResult.GetError())
	}
	if !strings.Contains(runner.prompt, "Profile the requested workload.") ||
		!strings.Contains(runner.prompt, "Measure allocations.") || strings.Contains(runner.prompt, "Use strict types.") {
		t.Fatalf("polyglot-performance prompt did not use its fixed Go skill bundle: %q", runner.prompt)
	}

	lastPrompt := runner.prompt
	invalidResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "agent_invoke",
		Arguments: map[string]any{
			"task":     "Try to select an unknown prompt",
			"agent":    "../outside-the-catalog",
			"language": "go",
		},
	})
	if err != nil {
		t.Fatalf("CallTool(invalid agent) error = %v", err)
	}
	if !invalidResult.IsError {
		t.Fatal("CallTool(invalid agent) did not return a tool error")
	}
	if runner.prompt != lastPrompt {
		t.Fatal("runner was called for an ID outside the configured catalog")
	}
}

func TestPolyglotAgentsResolveOneLanguageBundle(t *testing.T) {
	expectedSkills := map[string][]string{
		"typescript": {"typescript-core", "typescript-debugging", "typescript-formatting", "typescript-performance"},
		"go":         {"go-core", "go-debugging", "go-formatting", "go-performance"},
		"rust":       {"rust-core", "rust-debugging", "rust-formatting", "rust-performance"},
		"python":     {"python-core", "python-debugging", "python-formatting", "python-performance"},
	}
	for _, agentID := range polyglotAgents {
		for language, want := range expectedSkills {
			got, err := resolveAgentSkills(agentID, language)
			if err != nil {
				t.Fatalf("resolveAgentSkills(%q, %q) error = %v", agentID, language, err)
			}
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("resolveAgentSkills(%q, %q) = %v, want %v", agentID, language, got, want)
			}
		}
	}
	if _, err := resolveAgentSkills("polyglot-engineering", "javascript"); err == nil {
		t.Fatal("resolveAgentSkills() error = nil for unsupported language")
	}
}

func writeAsset(t *testing.T, root, relativePath, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
