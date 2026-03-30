package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/Masterminds/semver/v3"
	gh "github.com/google/go-github/v57/github"
	"github.com/nickromney-org/github-release-version-checker/pkg/types"
	"golang.org/x/oauth2"
)

// ClientOptions configures the GitHub API client.
type ClientOptions struct {
	Token     string
	Owner     string
	Repo      string
	Host      string
	BaseURL   string
	UploadURL string
}

// Client wraps the GitHub API client
type Client struct {
	gh      *gh.Client
	Owner   string
	Repo    string
	BaseURL string
}

// NewClient creates a new GitHub API client
func NewClient(token, owner, repo string) *Client {
	client, err := NewClientWithOptions(ClientOptions{
		Token: token,
		Owner: owner,
		Repo:  repo,
	})
	if err != nil {
		// github.com defaults should not fail; keep compatibility if an unexpected
		// URL normalisation bug slips through.
		return &Client{
			gh:    gh.NewClient(httpClient(token)),
			Owner: owner,
			Repo:  repo,
		}
	}
	return client
}

// NewClientWithOptions creates a GitHub API client with optional Enterprise host configuration.
func NewClientWithOptions(options ClientOptions) (*Client, error) {
	httpClient := httpClient(options.Token)
	ghClient := gh.NewClient(httpClient)

	baseURL := strings.TrimSpace(options.BaseURL)
	uploadURL := strings.TrimSpace(options.UploadURL)
	if baseURL == "" {
		baseURL = hostBaseURL(options.Host)
	}
	if uploadURL == "" && baseURL != "" {
		uploadURL = hostUploadURL(options.Host)
	}

	if baseURL != "" {
		var err error
		ghClient, err = ghClient.WithEnterpriseURLs(baseURL, uploadURL)
		if err != nil {
			return nil, fmt.Errorf("configure GitHub Enterprise client: %w", err)
		}
	}

	return &Client{
		gh:      ghClient,
		Owner:   options.Owner,
		Repo:    options.Repo,
		BaseURL: baseURL,
	}, nil
}

// GitHub returns the underlying go-github client for internal integrations.
func (c *Client) GitHub() *gh.Client {
	return c.gh
}

// GetLatestRelease fetches the latest release from GitHub
func (c *Client) GetLatestRelease(ctx context.Context) (*types.Release, error) {
	release, _, err := c.gh.Repositories.GetLatestRelease(ctx, c.Owner, c.Repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest release: %w", err)
	}

	return c.parseRelease(release)
}

// GetAllReleases fetches all releases from GitHub
func (c *Client) GetAllReleases(ctx context.Context) ([]types.Release, error) {
	var allReleases []types.Release

	opts := &gh.ListOptions{PerPage: 100}

	for page := 1; page <= 10; page++ { // Safety limit of 10 pages
		opts.Page = page

		releases, resp, err := c.gh.Repositories.ListReleases(ctx, c.Owner, c.Repo, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list releases (page %d): %w", page, err)
		}

		for _, ghRelease := range releases {
			// Skip drafts and prereleases
			if ghRelease.GetDraft() || ghRelease.GetPrerelease() {
				continue
			}

			release, err := c.parseRelease(ghRelease)
			if err != nil {
				// Log but don't fail - just skip invalid releases
				continue
			}

			allReleases = append(allReleases, *release)
		}

		// Check if we've reached the last page
		if resp.NextPage == 0 {
			break
		}
	}

	return allReleases, nil
}

// GetRecentReleases fetches only the N most recent releases
func (c *Client) GetRecentReleases(ctx context.Context, count int) ([]types.Release, error) {
	opts := &gh.ListOptions{PerPage: count}

	releases, _, err := c.gh.Repositories.ListReleases(ctx, c.Owner, c.Repo, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list recent releases: %w", err)
	}

	var result []types.Release
	for _, ghRelease := range releases {
		// Skip drafts and prereleases
		if ghRelease.GetDraft() || ghRelease.GetPrerelease() {
			continue
		}

		release, err := c.parseRelease(ghRelease)
		if err != nil {
			// Log but don't fail - just skip invalid releases
			continue
		}

		result = append(result, *release)
	}

	return result, nil
}

// parseRelease converts a GitHub release to our Release type
func (c *Client) parseRelease(ghRelease *gh.RepositoryRelease) (*types.Release, error) {
	tagName := ghRelease.GetTagName()
	if tagName == "" {
		return nil, fmt.Errorf("release has no tag name")
	}

	// Parse version (removing 'v' prefix if present)
	ver, err := semver.NewVersion(tagName)
	if err != nil {
		return nil, fmt.Errorf("invalid version %q: %w", tagName, err)
	}

	// Parse published date
	publishedAt := ghRelease.GetPublishedAt()
	if publishedAt.IsZero() {
		return nil, fmt.Errorf("release has no published date")
	}

	return &types.Release{
		Version:     ver,
		PublishedAt: publishedAt.Time,
		URL:         ghRelease.GetHTMLURL(),
	}, nil
}

func httpClient(token string) *http.Client {
	if token == "" {
		return http.DefaultClient
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	return oauth2.NewClient(context.Background(), ts)
}

func hostBaseURL(host string) string {
	host = strings.TrimSpace(host)
	if host == "" || host == "github.com" {
		return ""
	}
	if strings.Contains(host, "://") {
		return strings.TrimRight(host, "/")
	}
	return "https://" + host
}

func hostUploadURL(host string) string {
	baseURL := hostBaseURL(host)
	if baseURL == "" {
		return ""
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

// MockClient is a mock implementation for testing
type MockClient struct {
	LatestRelease *types.Release
	AllReleases   []types.Release
	Error         error
}

// GetLatestRelease returns the mocked latest release
func (m *MockClient) GetLatestRelease(ctx context.Context) (*types.Release, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return m.LatestRelease, nil
}

// GetAllReleases returns the mocked releases
func (m *MockClient) GetAllReleases(ctx context.Context) ([]types.Release, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return m.AllReleases, nil
}

// GetRecentReleases returns the first N mocked releases
func (m *MockClient) GetRecentReleases(ctx context.Context, count int) ([]types.Release, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	if len(m.AllReleases) <= count {
		return m.AllReleases, nil
	}
	return m.AllReleases[:count], nil
}
