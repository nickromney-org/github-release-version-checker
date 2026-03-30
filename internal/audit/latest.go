package audit

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	gh "github.com/google/go-github/v57/github"
)

type LatestResolveOptions struct {
	CooldownDays int
	Now          time.Time
}

type LatestVersion struct {
	Ref         string
	AgeDays     *int
	PublishedAt time.Time
}

type LatestResolver interface {
	ResolveLatest(ctx context.Context, kind Kind, name string, options LatestResolveOptions) (LatestVersion, error)
	ResolveSHA(ctx context.Context, kind Kind, name, ref string) (string, error)
}

type latestCandidate struct {
	Ref         string
	PublishedAt time.Time
}

type GitHubLatestResolver struct {
	Client *gh.Client

	mu          sync.Mutex
	latestCache map[string]LatestVersion
	shaCache    map[string]string
}

func NewGitHubLatestResolver(client *gh.Client) *GitHubLatestResolver {
	return &GitHubLatestResolver{
		Client:      client,
		latestCache: map[string]LatestVersion{},
		shaCache:    map[string]string{},
	}
}

func (r *GitHubLatestResolver) ResolveLatest(ctx context.Context, kind Kind, name string, options LatestResolveOptions) (LatestVersion, error) {
	if r == nil || r.Client == nil {
		return LatestVersion{}, nil
	}

	repo, ok := upstreamRepository(kind, name)
	if !ok {
		return LatestVersion{}, nil
	}

	now := options.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	cacheKey := latestCacheKey(repo, options.CooldownDays, now)

	r.mu.Lock()
	if cached, ok := r.latestCache[cacheKey]; ok {
		r.mu.Unlock()
		return cached, nil
	}
	r.mu.Unlock()

	owner, repoName, ok := strings.Cut(repo, "/")
	if !ok || owner == "" || repoName == "" {
		return LatestVersion{}, nil
	}

	latest, err := r.resolveLatestVersion(ctx, owner, repoName, LatestResolveOptions{
		CooldownDays: options.CooldownDays,
		Now:          now,
	})
	if err != nil {
		return LatestVersion{}, err
	}

	r.mu.Lock()
	r.latestCache[cacheKey] = latest
	r.mu.Unlock()

	return latest, nil
}

func (r *GitHubLatestResolver) ResolveSHA(ctx context.Context, kind Kind, name, ref string) (string, error) {
	if r == nil || r.Client == nil || ref == "" {
		return "", nil
	}

	repo, ok := upstreamRepository(kind, name)
	if !ok {
		return "", nil
	}

	cacheKey := shaCacheKey(kind, repo, ref)
	r.mu.Lock()
	if cached, ok := r.shaCache[cacheKey]; ok {
		r.mu.Unlock()
		return cached, nil
	}
	r.mu.Unlock()

	owner, repoName, ok := strings.Cut(repo, "/")
	if !ok || owner == "" || repoName == "" {
		return "", nil
	}

	sha, _, err := r.Client.Repositories.GetCommitSHA1(ctx, owner, repoName, ref, "")
	if err != nil {
		if isNotFound(err) {
			return "", nil
		}
		return "", fmt.Errorf("resolve sha for %s/%s@%s: %w", owner, repoName, ref, err)
	}

	r.mu.Lock()
	r.shaCache[cacheKey] = sha
	r.mu.Unlock()

	return sha, nil
}

func (r *GitHubLatestResolver) resolveLatestVersion(ctx context.Context, owner, repo string, options LatestResolveOptions) (LatestVersion, error) {
	releases, err := r.releaseCandidates(ctx, owner, repo)
	if err != nil {
		return LatestVersion{}, err
	}
	if len(releases) > 0 {
		return selectLatestCandidate(releases, options), nil
	}

	tags, err := r.tagCandidates(ctx, owner, repo)
	if err != nil {
		return LatestVersion{}, err
	}
	return selectLatestCandidate(tags, options), nil
}

func (r *GitHubLatestResolver) releaseCandidates(ctx context.Context, owner, repo string) ([]latestCandidate, error) {
	options := &gh.ListOptions{PerPage: 100}
	candidates := make([]latestCandidate, 0)

	for {
		releases, response, err := r.Client.Repositories.ListReleases(ctx, owner, repo, options)
		if err != nil {
			return nil, fmt.Errorf("list releases for %s/%s: %w", owner, repo, err)
		}
		for _, release := range releases {
			if release.GetDraft() || release.GetPrerelease() || release.GetTagName() == "" {
				continue
			}
			publishedAt := releasePublishedAt(release)
			if publishedAt.IsZero() {
				continue
			}
			candidates = append(candidates, latestCandidate{
				Ref:         release.GetTagName(),
				PublishedAt: publishedAt,
			})
		}
		if response == nil || response.NextPage == 0 {
			break
		}
		options.Page = response.NextPage
	}

	return candidates, nil
}

func (r *GitHubLatestResolver) tagCandidates(ctx context.Context, owner, repo string) ([]latestCandidate, error) {
	type semverCandidate struct {
		latestCandidate
		version     *semver.Version
		specificity int
	}

	options := &gh.ListOptions{PerPage: 100}
	candidates := make([]semverCandidate, 0)

	for {
		tags, response, err := r.Client.Repositories.ListTags(ctx, owner, repo, options)
		if err != nil {
			return nil, fmt.Errorf("list tags for %s/%s: %w", owner, repo, err)
		}
		for _, tag := range tags {
			version, specificity, ok := parseSemverTag(tag.GetName())
			if !ok {
				continue
			}
			publishedAt, err := r.tagPublishedAt(ctx, owner, repo, tag)
			if err != nil {
				return nil, err
			}
			if publishedAt.IsZero() {
				continue
			}
			candidates = append(candidates, semverCandidate{
				latestCandidate: latestCandidate{
					Ref:         tag.GetName(),
					PublishedAt: publishedAt,
				},
				version:     version,
				specificity: specificity,
			})
		}
		if response == nil || response.NextPage == 0 {
			break
		}
		options.Page = response.NextPage
	}

	slices.SortFunc(candidates, func(a, b semverCandidate) int {
		switch {
		case a.version.GreaterThan(b.version):
			return -1
		case b.version.GreaterThan(a.version):
			return 1
		case a.specificity > b.specificity:
			return -1
		case b.specificity > a.specificity:
			return 1
		default:
			return strings.Compare(a.Ref, b.Ref)
		}
	})

	result := make([]latestCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		result = append(result, candidate.latestCandidate)
	}

	return result, nil
}

func (r *GitHubLatestResolver) tagPublishedAt(ctx context.Context, owner, repo string, tag *gh.RepositoryTag) (time.Time, error) {
	if tag == nil || tag.Commit == nil || tag.Commit.GetSHA() == "" {
		return time.Time{}, nil
	}

	commit, _, err := r.Client.Repositories.GetCommit(ctx, owner, repo, tag.Commit.GetSHA(), nil)
	if err != nil {
		if isNotFound(err) {
			return time.Time{}, nil
		}
		return time.Time{}, fmt.Errorf("get commit for %s/%s tag %s: %w", owner, repo, tag.GetName(), err)
	}

	return commitPublishedAt(commit), nil
}

func selectLatestCandidate(candidates []latestCandidate, options LatestResolveOptions) LatestVersion {
	type eligibleCandidate struct {
		latestCandidate
		version     *semver.Version
		specificity int
	}

	now := options.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	cutoff := now.Add(-time.Duration(options.CooldownDays) * 24 * time.Hour)

	eligibleSemver := make([]eligibleCandidate, 0, len(candidates))
	var eligibleLatest latestCandidate

	for _, candidate := range candidates {
		if candidate.Ref == "" || candidate.PublishedAt.IsZero() {
			continue
		}
		if options.CooldownDays > 0 && candidate.PublishedAt.After(cutoff) {
			continue
		}

		if eligibleLatest.Ref == "" || candidate.PublishedAt.After(eligibleLatest.PublishedAt) {
			eligibleLatest = candidate
		}
		if version, specificity, ok := parseSemverTag(candidate.Ref); ok {
			eligibleSemver = append(eligibleSemver, eligibleCandidate{
				latestCandidate: candidate,
				version:         version,
				specificity:     specificity,
			})
		}
	}

	if len(eligibleSemver) > 0 {
		slices.SortFunc(eligibleSemver, func(a, b eligibleCandidate) int {
			switch {
			case a.version.GreaterThan(b.version):
				return -1
			case b.version.GreaterThan(a.version):
				return 1
			case a.specificity > b.specificity:
				return -1
			case b.specificity > a.specificity:
				return 1
			case a.PublishedAt.After(b.PublishedAt):
				return -1
			case b.PublishedAt.After(a.PublishedAt):
				return 1
			default:
				return strings.Compare(a.Ref, b.Ref)
			}
		})
		return latestVersionFromCandidate(now, eligibleSemver[0].latestCandidate)
	}

	if eligibleLatest.Ref != "" {
		return latestVersionFromCandidate(now, eligibleLatest)
	}

	return LatestVersion{}
}

func latestVersionFromCandidate(now time.Time, candidate latestCandidate) LatestVersion {
	ageDays := daysBetween(now, candidate.PublishedAt)
	return LatestVersion{
		Ref:         candidate.Ref,
		AgeDays:     intPtr(ageDays),
		PublishedAt: candidate.PublishedAt,
	}
}

func releasePublishedAt(release *gh.RepositoryRelease) time.Time {
	if release == nil {
		return time.Time{}
	}
	if release.PublishedAt != nil {
		return release.PublishedAt.UTC()
	}
	if release.CreatedAt != nil {
		return release.CreatedAt.UTC()
	}
	return time.Time{}
}

func commitPublishedAt(commit *gh.RepositoryCommit) time.Time {
	if commit == nil || commit.Commit == nil {
		return time.Time{}
	}
	if commit.Commit.Committer != nil && commit.Commit.Committer.Date != nil {
		return commit.Commit.Committer.Date.UTC()
	}
	if commit.Commit.Author != nil && commit.Commit.Author.Date != nil {
		return commit.Commit.Author.Date.UTC()
	}
	return time.Time{}
}

func daysBetween(now, then time.Time) int {
	if now.IsZero() || then.IsZero() || now.Before(then) {
		return 0
	}
	return int(now.Sub(then).Hours() / 24)
}

func upstreamRepository(kind Kind, name string) (string, bool) {
	switch kind {
	case KindAction:
		parts := strings.Split(name, "/")
		if len(parts) < 2 || parts[0] == "." || parts[0] == ".." {
			return "", false
		}
		return parts[0] + "/" + parts[1], true
	case KindReusableWorkflow:
		if strings.HasPrefix(name, "./") || strings.HasPrefix(name, "../") {
			return "", false
		}
		parts := strings.Split(name, "/")
		if len(parts) < 2 {
			return "", false
		}
		return parts[0] + "/" + parts[1], true
	default:
		return "", false
	}
}

func parseSemverTag(tag string) (*semver.Version, int, bool) {
	candidate := strings.TrimSpace(tag)
	if candidate == "" {
		return nil, 0, false
	}

	version, err := semver.NewVersion(candidate)
	if err != nil {
		return nil, 0, false
	}

	specificity := strings.Count(strings.TrimPrefix(candidate, "v"), ".")
	return version, specificity, true
}

func latestCacheKey(repo string, cooldownDays int, now time.Time) string {
	return fmt.Sprintf("%s:%d:%s", repo, cooldownDays, now.Format("2006-01-02"))
}

func shaCacheKey(kind Kind, repo, ref string) string {
	return string(kind) + ":" + repo + "@" + ref
}

func intPtr(value int) *int {
	return &value
}
