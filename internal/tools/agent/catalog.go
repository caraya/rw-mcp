package agent

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const (
	maxCatalogFiles       = 512
	maxCatalogFileBytes   = 1 << 20
	maxCatalogTotalBytes  = 16 << 20
	maxTaskBytes          = 64 << 10
	maxComposedPromptSize = 512 << 10
	maxSelectedResources  = 8
)

type Catalog struct {
	agents    map[string]catalogAsset
	skills    map[string]catalogAsset
	resources map[string]catalogAsset
}

type catalogAsset struct {
	id      string
	path    string
	title   string
	content string
}

type catalogAssetInfo struct {
	ID    string `json:"id" jsonschema:"Stable identifier to select this asset"`
	Title string `json:"title" jsonschema:"Human-readable asset title"`
	Path  string `json:"path" jsonschema:"Path within the configured agent-skills checkout"`
}

type catalogOutput struct {
	Agents    []catalogAssetInfo  `json:"agents" jsonschema:"Available agent prompts"`
	Skills    []catalogAssetInfo  `json:"skills" jsonschema:"Available skill instructions"`
	Resources []catalogAssetInfo  `json:"resources" jsonschema:"Available Markdown references and resources"`
	Profiles  []invocationProfile `json:"profiles" jsonschema:"Supported agent and language combinations with their fixed skill bundles"`
}

type invocationProfile struct {
	Agent    string   `json:"agent" jsonschema:"Agent identifier"`
	Language string   `json:"language,omitempty" jsonschema:"Required language for polyglot agents"`
	Skills   []string `json:"skills" jsonschema:"Skills automatically included for this profile"`
}

var polyglotAgents = []string{"polyglot-engineering", "polyglot-performance"}

var polyglotLanguageSkills = map[string][]string{
	"typescript": {"typescript-core", "typescript-debugging", "typescript-formatting", "typescript-performance"},
	"go":         {"go-core", "go-debugging", "go-formatting", "go-performance"},
	"rust":       {"rust-core", "rust-debugging", "rust-formatting", "rust-performance"},
	"python":     {"python-core", "python-debugging", "python-formatting", "python-performance"},
}

var fixedAgentSkills = map[string][]string{
	"content-editor": {"style-lookup", "code-example-validation"},
}

func LoadCatalog(directory string) (*Catalog, error) {
	root, err := filepath.Abs(directory)
	if err != nil {
		return nil, fmt.Errorf("resolve agent-skills directory: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("open agent-skills directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("agent-skills path %q is not a directory", directory)
	}

	catalog := &Catalog{
		agents:    make(map[string]catalogAsset),
		skills:    make(map[string]catalogAsset),
		resources: make(map[string]catalogAsset),
	}
	var fileCount int
	var totalBytes int64
	err = filepath.WalkDir(root, func(filename string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !entry.Type().IsRegular() || strings.ToLower(filepath.Ext(entry.Name())) != ".md" {
			return nil
		}
		fileCount++
		if fileCount > maxCatalogFiles {
			return fmt.Errorf("agent-skills catalog exceeds %d Markdown files", maxCatalogFiles)
		}
		relativePath, err := filepath.Rel(root, filename)
		if err != nil {
			return fmt.Errorf("resolve catalog asset path: %w", err)
		}
		assetPath := filepath.ToSlash(relativePath)
		content, err := readCatalogFile(filename)
		if err != nil {
			return fmt.Errorf("read catalog asset %q: %w", assetPath, err)
		}
		totalBytes += int64(len(content))
		if totalBytes > maxCatalogTotalBytes {
			return fmt.Errorf("agent-skills catalog exceeds %d bytes", maxCatalogTotalBytes)
		}
		kind, id := classifyCatalogAsset(assetPath)
		asset := catalogAsset{id: id, path: assetPath, title: markdownTitle(content, id), content: string(content)}
		var assets map[string]catalogAsset
		switch kind {
		case "agent":
			assets = catalog.agents
		case "skill":
			assets = catalog.skills
		default:
			assets = catalog.resources
		}
		if _, exists := assets[id]; exists {
			return fmt.Errorf("duplicate %s identifier %q in agent-skills catalog", kind, id)
		}
		assets[id] = asset
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan agent-skills directory: %w", err)
	}
	if fileCount == 0 {
		return nil, fmt.Errorf("agent-skills directory %q contains no Markdown files", directory)
	}
	return catalog, nil
}

func LoadCatalogAtCommit(directory, expectedSHA string) (*Catalog, error) {
	if strings.TrimSpace(expectedSHA) == "" {
		return LoadCatalog(directory)
	}
	root, err := filepath.Abs(directory)
	if err != nil {
		return nil, fmt.Errorf("resolve agent-skills directory: %w", err)
	}
	expectedSHA = strings.ToLower(strings.TrimSpace(expectedSHA))
	if !isFullGitSHA(expectedSHA) {
		return nil, fmt.Errorf("agent-skills SHA must be a full 40- or 64-character hexadecimal commit ID")
	}
	actualSHA, err := gitOutput(root, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return nil, fmt.Errorf("verify agent-skills Git revision: %w", err)
	}
	actualSHA = strings.ToLower(strings.TrimSpace(actualSHA))
	if actualSHA != expectedSHA {
		return nil, fmt.Errorf("agent-skills checkout is at %s, expected %s", actualSHA, expectedSHA)
	}
	workingTree, err := gitOutput(root, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return nil, fmt.Errorf("check agent-skills working tree: %w", err)
	}
	if strings.TrimSpace(workingTree) != "" {
		return nil, fmt.Errorf("agent-skills checkout at %s has modified or untracked files", expectedSHA)
	}
	ignoredFiles, err := gitOutput(root, "ls-files", "--others", "--ignored", "--exclude-standard", "-z")
	if err != nil {
		return nil, fmt.Errorf("check ignored agent-skills files: %w", err)
	}
	for _, filename := range strings.Split(ignoredFiles, "\x00") {
		if strings.EqualFold(filepath.Ext(filename), ".md") {
			return nil, fmt.Errorf("agent-skills checkout at %s contains ignored Markdown file %q", expectedSHA, filename)
		}
	}
	return LoadCatalog(root)
}

func isFullGitSHA(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}

func gitOutput(directory string, args ...string) (string, error) {
	commandArgs := append([]string{"-C", directory}, args...)
	output, err := exec.Command("git", commandArgs...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %v: %w: %s", args, err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func readCatalogFile(filename string) ([]byte, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, maxCatalogFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(content) > maxCatalogFileBytes {
		return nil, fmt.Errorf("file exceeds %d bytes", maxCatalogFileBytes)
	}
	return content, nil
}

func classifyCatalogAsset(assetPath string) (kind, id string) {
	cleanPath := filepath.ToSlash(filepath.Clean(filepath.FromSlash(assetPath)))
	if strings.HasPrefix(cleanPath, "agents/") {
		relative := strings.TrimPrefix(cleanPath, "agents/")
		if filepath.Base(relative) == "agent.md" {
			id = strings.TrimSuffix(filepath.ToSlash(filepath.Dir(relative)), ".")
			if id != "" {
				return "agent", id
			}
		}
		if !strings.Contains(relative, "/") && strings.ToLower(filepath.Ext(relative)) == ".md" {
			return "agent", strings.TrimSuffix(relative, filepath.Ext(relative))
		}
	}
	if strings.HasPrefix(cleanPath, "skills/") && filepath.Base(cleanPath) == "SKILL.md" {
		directory := filepath.ToSlash(filepath.Dir(strings.TrimPrefix(cleanPath, "skills/")))
		if directory != "." {
			return "skill", directory
		}
	}
	return "resource", cleanPath
}

func markdownTitle(content []byte, fallback string) string {
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if title, found := strings.CutPrefix(line, "# "); found {
			return strings.TrimSpace(title)
		}
	}
	return fallback
}

func (c *Catalog) list() catalogOutput {
	return catalogOutput{
		Agents:    catalogAssetInfos(c.agents),
		Skills:    catalogAssetInfos(c.skills),
		Resources: catalogAssetInfos(c.resources),
		Profiles:  c.invocationProfiles(),
	}
}

func (c *Catalog) invocationProfiles() []invocationProfile {
	profiles := make([]invocationProfile, 0, len(fixedAgentSkills)+len(polyglotAgents)*len(polyglotLanguageSkills))
	for agentID, skillIDs := range fixedAgentSkills {
		if _, exists := c.agents[agentID]; exists && c.hasSkills(skillIDs) {
			profiles = append(profiles, invocationProfile{Agent: agentID, Skills: append([]string(nil), skillIDs...)})
		}
	}
	for _, agentID := range polyglotAgents {
		if _, exists := c.agents[agentID]; !exists {
			continue
		}
		for _, language := range []string{"go", "python", "rust", "typescript"} {
			skillIDs := polyglotLanguageSkills[language]
			if c.hasSkills(skillIDs) {
				profiles = append(profiles, invocationProfile{
					Agent: agentID, Language: language, Skills: append([]string(nil), skillIDs...),
				})
			}
		}
	}
	return profiles
}

func (c *Catalog) hasSkills(skillIDs []string) bool {
	for _, id := range skillIDs {
		if _, exists := c.skills[id]; !exists {
			return false
		}
	}
	return true
}

func catalogAssetInfos(assets map[string]catalogAsset) []catalogAssetInfo {
	infos := make([]catalogAssetInfo, 0, len(assets))
	for _, asset := range assets {
		infos = append(infos, catalogAssetInfo{ID: asset.id, Title: asset.title, Path: asset.path})
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].ID < infos[j].ID })
	return infos
}

func (c *Catalog) composePrompt(task, agentID, language string, resourceIDs []string) (string, error) {
	task = strings.TrimSpace(task)
	if task == "" {
		return "", fmt.Errorf("task must not be empty")
	}
	if len(task) > maxTaskBytes {
		return "", fmt.Errorf("task exceeds %d bytes", maxTaskBytes)
	}
	skillIDs, err := resolveAgentSkills(agentID, language)
	if err != nil {
		return "", err
	}
	if len(resourceIDs) > maxSelectedResources {
		return "", fmt.Errorf("select no more than %d resources", maxSelectedResources)
	}

	var builder strings.Builder
	appendSection := func(kind, id string, asset catalogAsset) error {
		section := fmt.Sprintf("\n\n# %s: %s\nSource: %s\n\n%s", kind, id, asset.path, asset.content)
		if builder.Len()+len(section) > maxComposedPromptSize {
			return fmt.Errorf("selected agent instructions and resources exceed %d bytes", maxComposedPromptSize)
		}
		builder.WriteString(section)
		return nil
	}
	if agentID != "" {
		asset, ok := c.agents[agentID]
		if !ok {
			return "", fmt.Errorf("unknown agent %q; use agent_list_assets to list available IDs", agentID)
		}
		if err := appendSection("Agent", agentID, asset); err != nil {
			return "", err
		}
	}
	for _, id := range skillIDs {
		asset, ok := c.skills[id]
		if !ok {
			return "", fmt.Errorf("unknown skill %q; use agent_list_assets to list available IDs", id)
		}
		if err := appendSection("Skill", id, asset); err != nil {
			return "", err
		}
	}
	for _, id := range resourceIDs {
		asset, ok := c.resources[id]
		if !ok {
			return "", fmt.Errorf("unknown resource %q; use agent_list_assets to list available IDs", id)
		}
		if err := appendSection("Resource", id, asset); err != nil {
			return "", err
		}
	}

	taskSection := "\n\n# User Task\n\n" + task
	if builder.Len()+len(taskSection) > maxComposedPromptSize {
		return "", fmt.Errorf("composed agent prompt exceeds %d bytes", maxComposedPromptSize)
	}
	builder.WriteString(taskSection)
	return builder.String(), nil
}

func resolveAgentSkills(agentID, language string) ([]string, error) {
	if skills, ok := fixedAgentSkills[agentID]; ok {
		if language != "" {
			return nil, fmt.Errorf("agent %q does not take a language", agentID)
		}
		return skills, nil
	}
	if !contains(polyglotAgents, agentID) {
		return nil, fmt.Errorf("agent %q has no invocation policy; use agent_list_assets to inspect available agents", agentID)
	}
	language = strings.ToLower(strings.TrimSpace(language))
	skills, ok := polyglotLanguageSkills[language]
	if !ok {
		return nil, fmt.Errorf("agent %q requires language to be one of: go, python, rust, typescript", agentID)
	}
	return skills, nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
