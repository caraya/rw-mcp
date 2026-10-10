package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetRepositoryUsesAnonymousGitHubAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/repos/octocat/Hello-World" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("expected no Authorization header, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"full_name":        "octocat/Hello-World",
			"description":      "A sample repository",
			"html_url":         "https://github.com/octocat/Hello-World",
			"stargazers_count": 42,
		})
	}))
	defer server.Close()

	client := NewClient(server.Client(), "", server.URL)
	repository, err := client.GetRepository(context.Background(), "octocat", "Hello-World")
	if err != nil {
		t.Fatalf("GetRepository() error = %v", err)
	}
	if repository.FullName != "octocat/Hello-World" || repository.Stars != 42 {
		t.Fatalf("GetRepository() = %+v", repository)
	}
}

func TestGetAuthenticatedUserUsesConfiguredToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/user" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer test-token")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"login":"octocat","name":"Octocat"}`))
	}))
	defer server.Close()

	client := NewClient(server.Client(), "test-token", server.URL)
	user, err := client.GetAuthenticatedUser(context.Background())
	if err != nil {
		t.Fatalf("GetAuthenticatedUser() error = %v", err)
	}
	if user.Login != "octocat" {
		t.Fatalf("GetAuthenticatedUser() = %+v", user)
	}
}

func TestGetAuthenticatedUserRequiresToken(t *testing.T) {
	client := NewClient(nil, "", "https://api.github.com")
	if _, err := client.GetAuthenticatedUser(context.Background()); err == nil {
		t.Fatal("GetAuthenticatedUser() error = nil, want missing-token error")
	}
}
