package audit

import (
	"context"
	"errors"
	"fmt"
	"strings"

	gh "github.com/google/go-github/v57/github"
)

type GitHubRemoteSource struct {
	Client *gh.Client
}

func NewGitHubRemoteSource(client *gh.Client) *GitHubRemoteSource {
	return &GitHubRemoteSource{Client: client}
}

func (s *GitHubRemoteSource) GetRepository(ctx context.Context, owner, repo string) (Repository, error) {
	ghRepo, _, err := s.Client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return Repository{}, fmt.Errorf("get repository %s/%s: %w", owner, repo, err)
	}
	return mapGitHubRepository(ghRepo), nil
}

func (s *GitHubRemoteSource) ListRepositoriesByOwner(ctx context.Context, owner, visibility string) ([]Repository, error) {
	account, _, err := s.Client.Users.Get(ctx, owner)
	if err != nil {
		return nil, fmt.Errorf("get owner %s: %w", owner, err)
	}

	if strings.EqualFold(account.GetType(), "Organization") {
		return s.ListRepositoriesByOrg(ctx, owner)
	}

	authLogin, err := s.authenticatedUserLogin(ctx)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(authLogin, owner) {
		return s.listRepositoriesByAuthenticatedOwner(ctx, visibility)
	}

	switch strings.TrimSpace(visibility) {
	case "", "public":
		return s.listRepositoriesByUser(ctx, owner)
	case "private", "all", "internal":
		return nil, fmt.Errorf("private or full scans for user owner %s require a token for that same owner; use --visibility public or authenticate as %s", owner, owner)
	default:
		return nil, fmt.Errorf("invalid visibility %q for owner %s", visibility, owner)
	}
}

func (s *GitHubRemoteSource) ListRepositoriesByOrg(ctx context.Context, org string) ([]Repository, error) {
	options := &gh.RepositoryListByOrgOptions{
		Type: "all",
		ListOptions: gh.ListOptions{
			PerPage: 100,
		},
	}

	var repositories []Repository
	for {
		repos, response, err := s.Client.Repositories.ListByOrg(ctx, org, options)
		if err != nil {
			return nil, fmt.Errorf("list repositories for org %s: %w", org, err)
		}
		for _, repo := range repos {
			repositories = append(repositories, mapGitHubRepository(repo))
		}
		if response.NextPage == 0 {
			break
		}
		options.Page = response.NextPage
	}

	return repositories, nil
}

func (s *GitHubRemoteSource) listRepositoriesByUser(ctx context.Context, owner string) ([]Repository, error) {
	options := &gh.RepositoryListByUserOptions{
		Type:      "owner",
		Sort:      "updated",
		Direction: "desc",
		ListOptions: gh.ListOptions{
			PerPage: 100,
		},
	}

	var repositories []Repository
	for {
		repos, response, err := s.Client.Repositories.ListByUser(ctx, owner, options)
		if err != nil {
			return nil, fmt.Errorf("list public repositories for owner %s: %w", owner, err)
		}
		for _, repo := range repos {
			repositories = append(repositories, mapGitHubRepository(repo))
		}
		if response.NextPage == 0 {
			break
		}
		options.Page = response.NextPage
	}

	return repositories, nil
}

func (s *GitHubRemoteSource) listRepositoriesByAuthenticatedOwner(ctx context.Context, visibility string) ([]Repository, error) {
	if visibility == "internal" {
		return nil, fmt.Errorf("visibility internal is not supported for user owners")
	}

	options := &gh.RepositoryListByAuthenticatedUserOptions{
		Affiliation: "owner",
		ListOptions: gh.ListOptions{
			PerPage: 100,
		},
	}
	if strings.TrimSpace(visibility) != "" {
		options.Visibility = visibility
	} else {
		options.Visibility = "all"
	}

	var repositories []Repository
	for {
		repos, response, err := s.Client.Repositories.ListByAuthenticatedUser(ctx, options)
		if err != nil {
			return nil, fmt.Errorf("list repositories for authenticated owner: %w", err)
		}
		for _, repo := range repos {
			repositories = append(repositories, mapGitHubRepository(repo))
		}
		if response.NextPage == 0 {
			break
		}
		options.Page = response.NextPage
	}

	return repositories, nil
}

func (s *GitHubRemoteSource) authenticatedUserLogin(ctx context.Context) (string, error) {
	user, _, err := s.Client.Users.Get(ctx, "")
	if err != nil {
		if isUnauthenticated(err) {
			return "", nil
		}
		return "", fmt.Errorf("get authenticated user: %w", err)
	}
	return user.GetLogin(), nil
}

func (s *GitHubRemoteSource) ListWorkflowFiles(ctx context.Context, repo Repository) ([]WorkflowFile, error) {
	options := &gh.RepositoryContentGetOptions{}
	if repo.DefaultBranch != "" {
		options.Ref = repo.DefaultBranch
	}

	owner, repoName, ok := strings.Cut(repo.FullName, "/")
	if !ok {
		return nil, fmt.Errorf("repository %q does not have a full name", repo.FullName)
	}

	_, directoryContent, _, err := s.Client.Repositories.GetContents(ctx, owner, repoName, ".github/workflows", options)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list workflow files for %s: %w", repo.FullName, err)
	}

	files := make([]WorkflowFile, 0, len(directoryContent))
	for _, entry := range directoryContent {
		if entry.GetType() != "file" {
			continue
		}
		entryName := entry.GetName()
		if !strings.HasSuffix(entryName, ".yml") && !strings.HasSuffix(entryName, ".yaml") {
			continue
		}

		fileContent, _, _, err := s.Client.Repositories.GetContents(ctx, owner, repoName, entry.GetPath(), options)
		if err != nil {
			return nil, fmt.Errorf("read workflow %s in %s: %w", entry.GetPath(), repo.FullName, err)
		}
		decoded, err := fileContent.GetContent()
		if err != nil {
			return nil, fmt.Errorf("decode workflow %s in %s: %w", entry.GetPath(), repo.FullName, err)
		}

		files = append(files, WorkflowFile{
			Path:    entry.GetPath(),
			Content: []byte(decoded),
		})
	}

	return files, nil
}

func mapGitHubRepository(repo *gh.Repository) Repository {
	visibility := repo.GetVisibility()
	if visibility == "" {
		if repo.GetPrivate() {
			visibility = "private"
		} else {
			visibility = "public"
		}
	}

	return Repository{
		Name:          repo.GetName(),
		FullName:      repo.GetFullName(),
		DefaultBranch: repo.GetDefaultBranch(),
		Archived:      repo.GetArchived(),
		Fork:          repo.GetFork(),
		Language:      repo.GetLanguage(),
		Visibility:    visibility,
		Source:        "remote",
	}
}

func isNotFound(err error) bool {
	var responseError *gh.ErrorResponse
	return errors.As(err, &responseError) && responseError.Response != nil && responseError.Response.StatusCode == 404
}

func isUnauthenticated(err error) bool {
	var responseError *gh.ErrorResponse
	return errors.As(err, &responseError) && responseError.Response != nil && responseError.Response.StatusCode == 401
}
