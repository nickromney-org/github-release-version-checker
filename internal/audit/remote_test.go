package audit

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gh "github.com/google/go-github/v57/github"
)

func TestGitHubRemoteSourcePaginationAndWorkflowLoading(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/api/v3/orgs/acme/repos", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("page") {
		case "2":
			fmt.Fprint(w, `[{"name":"backend-api-two","full_name":"acme/backend-api-two","default_branch":"main","visibility":"private","language":"Go"}]`)
		default:
			w.Header().Set("Link", fmt.Sprintf(`<%s/api/v3/orgs/acme/repos?page=2>; rel="next"`, server.URL))
			fmt.Fprint(w, `[{"name":"backend-api-one","full_name":"acme/backend-api-one","default_branch":"main","visibility":"private","language":"Go"}]`)
		}
	})

	mux.HandleFunc("/api/v3/repos/acme/backend-api-one/contents/.github/workflows", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"type":"file","name":"ci.yml","path":".github/workflows/ci.yml"}]`)
	})

	mux.HandleFunc("/api/v3/repos/acme/backend-api-one/contents/.github/workflows/ci.yml", func(w http.ResponseWriter, r *http.Request) {
		content := base64.StdEncoding.EncodeToString([]byte("name: ci\njobs:\n  test:\n    steps:\n      - uses: actions/checkout@v4\n"))
		fmt.Fprintf(w, `{"type":"file","name":"ci.yml","path":".github/workflows/ci.yml","encoding":"base64","content":"%s"}`, content)
	})

	client, err := gh.NewClient(server.Client()).WithEnterpriseURLs(server.URL, server.URL)
	if err != nil {
		t.Fatalf("WithEnterpriseURLs() error = %v", err)
	}

	source := NewGitHubRemoteSource(client)
	repositories, err := source.ListRepositoriesByOrg(context.Background(), "acme")
	if err != nil {
		t.Fatalf("ListRepositoriesByOrg() error = %v", err)
	}

	if len(repositories) != 2 {
		t.Fatalf("got %d repositories, want 2", len(repositories))
	}

	workflows, err := source.ListWorkflowFiles(context.Background(), repositories[0])
	if err != nil {
		t.Fatalf("ListWorkflowFiles() error = %v", err)
	}
	if len(workflows) != 1 {
		t.Fatalf("got %d workflow files, want 1", len(workflows))
	}
	if !strings.Contains(string(workflows[0].Content), "actions/checkout@v4") {
		t.Fatalf("unexpected workflow content: %s", string(workflows[0].Content))
	}
}

func TestGitHubRemoteSourceListRepositoriesByOwnerPublicUser(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/api/v3/users/nick", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"login":"nick","type":"User"}`)
	})
	mux.HandleFunc("/api/v3/user", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Requires authentication"}`, http.StatusUnauthorized)
	})
	mux.HandleFunc("/api/v3/users/nick/repos", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"name":"public-repo","full_name":"nick/public-repo","default_branch":"main","visibility":"public","language":"Go"}]`)
	})

	client, err := gh.NewClient(server.Client()).WithEnterpriseURLs(server.URL, server.URL)
	if err != nil {
		t.Fatalf("WithEnterpriseURLs() error = %v", err)
	}

	source := NewGitHubRemoteSource(client)
	repositories, err := source.ListRepositoriesByOwner(context.Background(), "nick", "")
	if err != nil {
		t.Fatalf("ListRepositoriesByOwner() error = %v", err)
	}
	if len(repositories) != 1 {
		t.Fatalf("got %d repositories, want 1", len(repositories))
	}
	if repositories[0].FullName != "nick/public-repo" {
		t.Fatalf("FullName = %q, want nick/public-repo", repositories[0].FullName)
	}
}

func TestGitHubRemoteSourceListRepositoriesByOwnerAuthenticatedUser(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/api/v3/users/nick", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"login":"nick","type":"User"}`)
	})
	mux.HandleFunc("/api/v3/user", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"login":"nick","type":"User"}`)
	})
	mux.HandleFunc("/api/v3/user/repos", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("visibility"); got != "all" {
			t.Fatalf("visibility = %q, want all", got)
		}
		if got := r.URL.Query().Get("affiliation"); got != "owner" {
			t.Fatalf("affiliation = %q, want owner", got)
		}
		if got := r.URL.Query().Get("type"); got != "" {
			t.Fatalf("type = %q, want empty", got)
		}
		fmt.Fprint(w, `[{"name":"private-repo","full_name":"nick/private-repo","default_branch":"main","visibility":"private","language":"Go"}]`)
	})

	client, err := gh.NewClient(server.Client()).WithEnterpriseURLs(server.URL, server.URL)
	if err != nil {
		t.Fatalf("WithEnterpriseURLs() error = %v", err)
	}

	source := NewGitHubRemoteSource(client)
	repositories, err := source.ListRepositoriesByOwner(context.Background(), "nick", "")
	if err != nil {
		t.Fatalf("ListRepositoriesByOwner() error = %v", err)
	}
	if len(repositories) != 1 {
		t.Fatalf("got %d repositories, want 1", len(repositories))
	}
	if repositories[0].Visibility != "private" {
		t.Fatalf("Visibility = %q, want private", repositories[0].Visibility)
	}
}

func TestGitHubRemoteSourceListRepositoriesByOwnerPrivateRequiresMatchingToken(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/api/v3/users/nick", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"login":"nick","type":"User"}`)
	})
	mux.HandleFunc("/api/v3/user", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Requires authentication"}`, http.StatusUnauthorized)
	})

	client, err := gh.NewClient(server.Client()).WithEnterpriseURLs(server.URL, server.URL)
	if err != nil {
		t.Fatalf("WithEnterpriseURLs() error = %v", err)
	}

	source := NewGitHubRemoteSource(client)
	_, err = source.ListRepositoriesByOwner(context.Background(), "nick", "all")
	if err == nil || !strings.Contains(err.Error(), "require a token for that same owner") {
		t.Fatalf("expected private owner scan error, got %v", err)
	}
}
