package client

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	gh "github.com/google/go-github/v57/github"
	"github.com/nickromney-org/github-release-version-checker/pkg/types"
)

// TestNewClient tests client creation
func TestNewClient(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{
			name:  "client with token",
			token: "ghp_test123",
		},
		{
			name:  "client without token",
			token: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.token, "actions", "runner")
			if client == nil {
				t.Fatal("NewClient returned nil")
			}
			if client.gh == nil {
				t.Fatal("client.gh is nil")
			}
			if client.Owner != "actions" {
				t.Errorf("Owner = %v, want actions", client.Owner)
			}
			if client.Repo != "runner" {
				t.Errorf("Repo = %v, want runner", client.Repo)
			}
		})
	}
}

func TestNewClientWithOptionsEnterpriseHost(t *testing.T) {
	client, err := NewClientWithOptions(ClientOptions{
		Token: "ghp_test123",
		Host:  "github.example.com",
		Owner: "acme",
		Repo:  "backend-api",
	})
	if err != nil {
		t.Fatalf("NewClientWithOptions() error = %v", err)
	}
	if client == nil || client.gh == nil {
		t.Fatal("expected client and underlying gh client")
	}
	if client.Owner != "acme" || client.Repo != "backend-api" {
		t.Fatalf("unexpected client owner/repo: %s/%s", client.Owner, client.Repo)
	}
	if client.BaseURL != "https://github.example.com" {
		t.Fatalf("BaseURL = %q, want https://github.example.com", client.BaseURL)
	}
}

// TestParseRelease tests release parsing
func TestParseRelease(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		release   *gh.RepositoryRelease
		wantErr   bool
		wantVer   string
		checkURL  bool
		expectURL string
	}{
		{
			name: "valid release",
			release: &gh.RepositoryRelease{
				TagName:     stringPtr("v2.329.0"),
				PublishedAt: &gh.Timestamp{Time: now},
				HTMLURL:     stringPtr("https://github.com/actions/runner/releases/tag/v2.329.0"),
			},
			wantErr:   false,
			wantVer:   "2.329.0",
			checkURL:  true,
			expectURL: "https://github.com/actions/runner/releases/tag/v2.329.0",
		},
		{
			name: "version without v prefix",
			release: &gh.RepositoryRelease{
				TagName:     stringPtr("2.329.0"),
				PublishedAt: &gh.Timestamp{Time: now},
				HTMLURL:     stringPtr("https://github.com/actions/runner/releases/tag/v2.329.0"),
			},
			wantErr: false,
			wantVer: "2.329.0",
		},
		{
			name: "missing tag name",
			release: &gh.RepositoryRelease{
				PublishedAt: &gh.Timestamp{Time: now},
				HTMLURL:     stringPtr("https://github.com/actions/runner/releases/tag/v2.329.0"),
			},
			wantErr: true,
		},
		{
			name: "invalid version",
			release: &gh.RepositoryRelease{
				TagName:     stringPtr("not-a-version"),
				PublishedAt: &gh.Timestamp{Time: now},
				HTMLURL:     stringPtr("https://github.com/actions/runner/releases/tag/invalid"),
			},
			wantErr: true,
		},
		{
			name: "missing published date",
			release: &gh.RepositoryRelease{
				TagName: stringPtr("v2.329.0"),
				HTMLURL: stringPtr("https://github.com/actions/runner/releases/tag/v2.329.0"),
			},
			wantErr: true,
		},
	}

	client := NewClient("", "actions", "runner")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			release, err := client.parseRelease(tt.release)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if release == nil {
				t.Fatal("release is nil")
			}

			if release.Version.String() != tt.wantVer {
				t.Errorf("version = %v, want %v", release.Version.String(), tt.wantVer)
			}

			if tt.checkURL && release.URL != tt.expectURL {
				t.Errorf("URL = %v, want %v", release.URL, tt.expectURL)
			}

			if release.PublishedAt.IsZero() {
				t.Error("PublishedAt is zero")
			}
		})
	}
}

// TestMockClient tests the mock client implementation
func TestMockClient(t *testing.T) {
	ctx := context.Background()

	t.Run("GetLatestRelease success", func(t *testing.T) {
		expected := newTestRelease("2.329.0", "actions", "runner", 0)
		mock := &MockClient{
			LatestRelease: &expected,
		}

		release, err := mock.GetLatestRelease(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if release.Version.String() != "2.329.0" {
			t.Errorf("version = %v, want 2.329.0", release.Version.String())
		}
	})

	t.Run("GetLatestRelease error", func(t *testing.T) {
		expectedErr := fmt.Errorf("test error")
		mock := &MockClient{
			Error: expectedErr,
		}

		_, err := mock.GetLatestRelease(ctx)
		if err != expectedErr {
			t.Errorf("error = %v, want %v", err, expectedErr)
		}
	})

	t.Run("GetAllReleases success", func(t *testing.T) {
		releases := []types.Release{
			newTestRelease("2.329.0", "actions", "runner", 0),
			newTestRelease("2.328.0", "actions", "runner", 5),
			newTestRelease("2.327.1", "actions", "runner", 10),
		}
		mock := &MockClient{
			AllReleases: releases,
		}

		result, err := mock.GetAllReleases(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 3 {
			t.Errorf("got %d releases, want 3", len(result))
		}
	})

	t.Run("GetAllReleases error", func(t *testing.T) {
		expectedErr := fmt.Errorf("test error")
		mock := &MockClient{
			Error: expectedErr,
		}

		_, err := mock.GetAllReleases(ctx)
		if err != expectedErr {
			t.Errorf("error = %v, want %v", err, expectedErr)
		}
	})

	t.Run("GetRecentReleases success", func(t *testing.T) {
		releases := []types.Release{
			newTestRelease("2.329.0", "actions", "runner", 0),
			newTestRelease("2.328.0", "actions", "runner", 5),
			newTestRelease("2.327.1", "actions", "runner", 10),
			newTestRelease("2.327.0", "actions", "runner", 15),
		}
		mock := &MockClient{
			AllReleases: releases,
		}

		// Request 2 releases
		result, err := mock.GetRecentReleases(ctx, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("got %d releases, want 2", len(result))
		}
		if result[0].Version.String() != "2.329.0" {
			t.Errorf("first release = %v, want 2.329.0", result[0].Version.String())
		}
	})

	t.Run("GetRecentReleases all releases", func(t *testing.T) {
		releases := []types.Release{
			newTestRelease("2.329.0", "actions", "runner", 0),
			newTestRelease("2.328.0", "actions", "runner", 5),
		}
		mock := &MockClient{
			AllReleases: releases,
		}

		// Request more releases than available
		result, err := mock.GetRecentReleases(ctx, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("got %d releases, want 2", len(result))
		}
	})

	t.Run("GetRecentReleases error", func(t *testing.T) {
		expectedErr := fmt.Errorf("test error")
		mock := &MockClient{
			Error: expectedErr,
		}

		_, err := mock.GetRecentReleases(ctx, 5)
		if err != expectedErr {
			t.Errorf("error = %v, want %v", err, expectedErr)
		}
	})
}

// TestNewTestRelease tests the test helper function
func TestNewTestRelease(t *testing.T) {
	tests := []struct {
		name       string
		version    string
		daysAgo    int
		wantPanic  bool
		expectVer  string
		expectURL  string
		checkAge   bool
		minDaysAgo int
		maxDaysAgo int
	}{
		{
			name:       "current version",
			version:    "2.329.0",
			daysAgo:    0,
			expectVer:  "2.329.0",
			expectURL:  "https://github.com/actions/runner/releases/tag/v2.329.0",
			checkAge:   true,
			minDaysAgo: 0,
			maxDaysAgo: 1,
		},
		{
			name:       "old version",
			version:    "2.327.1",
			daysAgo:    30,
			expectVer:  "2.327.1",
			expectURL:  "https://github.com/actions/runner/releases/tag/v2.327.1",
			checkAge:   true,
			minDaysAgo: 29,
			maxDaysAgo: 31,
		},
		{
			name:      "invalid version",
			version:   "not-a-version",
			daysAgo:   0,
			wantPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantPanic {
				defer func() {
					if r := recover(); r == nil {
						t.Error("expected panic, got none")
					}
				}()
			}

			release := newTestRelease(tt.version, "actions", "runner", tt.daysAgo)

			if tt.wantPanic {
				return
			}

			if release.Version.String() != tt.expectVer {
				t.Errorf("version = %v, want %v", release.Version.String(), tt.expectVer)
			}

			if release.URL != tt.expectURL {
				t.Errorf("URL = %v, want %v", release.URL, tt.expectURL)
			}

			if tt.checkAge {
				// Check that PublishedAt is approximately correct
				expectedTime := time.Now().AddDate(0, 0, -tt.daysAgo)
				daysDiff := int(time.Since(release.PublishedAt).Hours() / 24)
				if daysDiff < tt.minDaysAgo || daysDiff > tt.maxDaysAgo {
					t.Errorf("release age = %d days, want between %d and %d days",
						daysDiff, tt.minDaysAgo, tt.maxDaysAgo)
				}

				// Verify it's not zero
				if release.PublishedAt.IsZero() {
					t.Error("PublishedAt is zero")
				}

				// Verify it's not in the future
				if release.PublishedAt.After(expectedTime.Add(time.Second)) {
					t.Error("PublishedAt is in the future")
				}
			}
		})
	}
}

// Helper function to create test releases
func newTestRelease(versionStr, owner, repo string, daysAgo int) types.Release {
	v := semver.MustParse(versionStr)
	return types.Release{
		Version:     v,
		PublishedAt: time.Now().AddDate(0, 0, -daysAgo),
		URL:         fmt.Sprintf("https://github.com/%s/%s/releases/tag/v%s", owner, repo, versionStr),
	}
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}
