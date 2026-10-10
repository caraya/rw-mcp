package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

const maxResponseBytes = 1 << 20

var pathSegmentPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

type Client struct {
	httpClient *http.Client
	token      string
	baseURL    string
}

type Repository struct {
	FullName    string `json:"full_name"`
	Description string `json:"description"`
	HTMLURL     string `json:"html_url"`
	Stars       int    `json:"stargazers_count"`
}

type User struct {
	Login string `json:"login"`
	Name  string `json:"name"`
	URL   string `json:"html_url"`
}

func NewClient(httpClient *http.Client, token, baseURL string) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	return &Client{
		httpClient: httpClient,
		token:      token,
		baseURL:    strings.TrimRight(baseURL, "/"),
	}
}

func (c *Client) GetRepository(ctx context.Context, owner, repository string) (Repository, error) {
	var result Repository
	if !validPathSegment(owner) || !validPathSegment(repository) {
		return result, errors.New("owner and repository must be valid GitHub path segments")
	}
	err := c.getJSON(ctx, "/repos/"+owner+"/"+repository, false, &result)
	return result, err
}

func (c *Client) GetAuthenticatedUser(ctx context.Context) (User, error) {
	var result User
	if strings.TrimSpace(c.token) == "" {
		return result, errors.New("GITHUB_TOKEN is required for the authenticated GitHub API example")
	}
	err := c.getJSON(ctx, "/user", true, &result)
	return result, err
}

func (c *Client) getJSON(ctx context.Context, path string, authenticated bool, result any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("create GitHub API request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if authenticated {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("call GitHub API %s: %w", path, err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("GitHub API %s returned %s", path, response.Status)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read GitHub API response %s: %w", path, err)
	}
	if len(body) > maxResponseBytes {
		return fmt.Errorf("GitHub API response %s exceeds %d bytes", path, maxResponseBytes)
	}
	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("decode GitHub API response %s: %w", path, err)
	}
	return nil
}

func validPathSegment(value string) bool {
	return value != "." && value != ".." && pathSegmentPattern.MatchString(value)
}
