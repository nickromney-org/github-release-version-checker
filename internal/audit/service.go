package audit

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type RemoteSource interface {
	GetRepository(ctx context.Context, owner, repo string) (Repository, error)
	ListRepositoriesByOwner(ctx context.Context, owner, visibility string) ([]Repository, error)
	ListRepositoriesByOrg(ctx context.Context, org string) ([]Repository, error)
	ListWorkflowFiles(ctx context.Context, repo Repository) ([]WorkflowFile, error)
}

type Scanner struct {
	Remote RemoteSource
	Latest LatestResolver
	Now    func() time.Time
}

func NewScanner(remote RemoteSource) *Scanner {
	return &Scanner{Remote: remote}
}

func (s *Scanner) ScanLocal(ctx context.Context, options Options) (*Result, error) {
	repositories, err := DiscoverLocalRepositories(options.Path, options.MaxDepth)
	if err != nil {
		return nil, err
	}

	scans := make([]RepositoryScan, 0, len(repositories))
	for _, repo := range repositories {
		if !matchesRepoFilters(repo, options.RepoFilters) {
			continue
		}
		workflows, err := LoadLocalWorkflowFiles(repo)
		if err != nil {
			return nil, err
		}
		scans = append(scans, RepositoryScan{Repository: repo, Workflows: workflows})
	}

	return s.scanRepositories(ctx, options, scans)
}

func (s *Scanner) ScanRepo(ctx context.Context, options Options) (*Result, error) {
	if s.Remote == nil {
		return nil, fmt.Errorf("remote source is not configured")
	}

	owner, repo, ok := strings.Cut(options.Repo, "/")
	if !ok || owner == "" || repo == "" {
		return nil, fmt.Errorf("invalid repository %q: expected owner/repo", options.Repo)
	}

	repository, err := s.Remote.GetRepository(ctx, owner, repo)
	if err != nil {
		return nil, err
	}
	if !matchesRepoFilters(repository, options.RepoFilters) {
		return s.scanRepositories(ctx, options, nil)
	}

	workflows, err := s.Remote.ListWorkflowFiles(ctx, repository)
	if err != nil {
		return nil, err
	}

	return s.scanRepositories(ctx, options, []RepositoryScan{{
		Repository: repository,
		Workflows:  workflows,
	}})
}

func (s *Scanner) ScanOrg(ctx context.Context, options Options) (*Result, error) {
	if s.Remote == nil {
		return nil, fmt.Errorf("remote source is not configured")
	}

	repositories, err := s.Remote.ListRepositoriesByOrg(ctx, options.Org)
	if err != nil {
		return nil, err
	}

	filtered := make([]Repository, 0, len(repositories))
	for _, repo := range repositories {
		if !matchesRemoteRepo(repo, options) {
			continue
		}
		filtered = append(filtered, repo)
	}

	scans := make([]RepositoryScan, 0, len(filtered))
	for _, repo := range filtered {
		workflows, err := s.Remote.ListWorkflowFiles(ctx, repo)
		if err != nil {
			return nil, err
		}
		scans = append(scans, RepositoryScan{Repository: repo, Workflows: workflows})
	}

	return s.scanRepositories(ctx, options, scans)
}

func (s *Scanner) ScanOwner(ctx context.Context, options Options) (*Result, error) {
	if s.Remote == nil {
		return nil, fmt.Errorf("remote source is not configured")
	}

	repositories, err := s.Remote.ListRepositoriesByOwner(ctx, options.Owner, options.Visibility)
	if err != nil {
		return nil, err
	}

	filtered := make([]Repository, 0, len(repositories))
	for _, repo := range repositories {
		if !matchesRemoteRepo(repo, options) {
			continue
		}
		filtered = append(filtered, repo)
	}

	scans := make([]RepositoryScan, 0, len(filtered))
	for _, repo := range filtered {
		workflows, err := s.Remote.ListWorkflowFiles(ctx, repo)
		if err != nil {
			return nil, err
		}
		scans = append(scans, RepositoryScan{Repository: repo, Workflows: workflows})
	}

	return s.scanRepositories(ctx, options, scans)
}

func (s *Scanner) scanRepositories(ctx context.Context, options Options, scans []RepositoryScan) (*Result, error) {
	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now().UTC()
	}

	var (
		workflowFilesScanned  int
		repositoriesWithFlows int
		occurrences           []Occurrence
		warnings              []Warning
	)

	for _, scan := range scans {
		if len(scan.Workflows) > 0 {
			repositoriesWithFlows++
		}
		workflowFilesScanned += len(scan.Workflows)

		for _, workflow := range scan.Workflows {
			parsed, err := ParseWorkflow(scan.Repository, workflow.Path, workflow.Content)
			if err != nil {
				warnings = append(warnings, Warning{
					File:    workflowLocation(scan.Repository, workflow.Path),
					Message: err.Error(),
				})
				continue
			}
			for _, occurrence := range parsed {
				if !matchesOccurrence(occurrence, options) {
					continue
				}
				occurrences = append(occurrences, occurrence)
			}
		}
	}

	summary := buildSummary(occurrences)
	if (options.ResolveLatest || options.ResolveSHA) && s.Latest != nil {
		warnings = append(warnings, s.populateLatestRefs(ctx, summary, options, now)...)
	}
	populateSummaryLinks(summary, options.Host)
	enrichOccurrencesFromSummary(occurrences, summary, options)

	return &Result{
		Metadata: Metadata{
			GeneratedAt:           now,
			Mode:                  options.Mode,
			Path:                  options.Path,
			Repo:                  options.Repo,
			Owner:                 options.Owner,
			Org:                   options.Org,
			Host:                  options.Host,
			RepositoriesScanned:   len(scans),
			RepositoriesWithFlows: repositoriesWithFlows,
			WorkflowFilesScanned:  workflowFilesScanned,
			OccurrencesFound:      len(occurrences),
			WarningsCount:         len(warnings),
			Filters:               options,
		},
		Summary:     summary,
		Occurrences: occurrences,
		Warnings:    warnings,
	}, nil
}

func (s *Scanner) populateLatestRefs(ctx context.Context, summary []SummaryRow, options Options, now time.Time) []Warning {
	warnings := make([]Warning, 0)
	for i := range summary {
		if options.ResolveLatest || options.ResolveSHA {
			latest, err := s.Latest.ResolveLatest(ctx, summary[i].Kind, summary[i].Name, LatestResolveOptions{
				CooldownDays: options.CooldownDays,
				Now:          now,
			})
			if err != nil {
				warnings = append(warnings, Warning{
					File:    summary[i].Name,
					Message: err.Error(),
				})
			} else {
				summary[i].LatestRef = latest.Ref
				summary[i].LatestAgeDays = latest.AgeDays
			}
		}

		if options.ResolveSHA && summary[i].LatestRef != "" {
			latestSHA, err := s.Latest.ResolveSHA(ctx, summary[i].Kind, summary[i].Name, summary[i].LatestRef)
			if err != nil {
				warnings = append(warnings, Warning{
					File:    summary[i].Name,
					Message: err.Error(),
				})
				continue
			}
			summary[i].LatestSHA = latestSHA
		}
	}
	return warnings
}

func enrichOccurrencesFromSummary(occurrences []Occurrence, summary []SummaryRow, options Options) {
	summaryByKey := make(map[summaryMatchKey]SummaryRow, len(summary))
	for _, row := range summary {
		summaryByKey[summaryKey(row.Kind, row.Name, row.Ref)] = row
	}

	for i := range occurrences {
		row, ok := summaryByKey[summaryKey(occurrences[i].Kind, occurrences[i].Name, occurrences[i].Ref)]
		if !ok {
			continue
		}
		occurrences[i].LatestRef = row.LatestRef
		occurrences[i].UpstreamURL = row.UpstreamURL
		occurrences[i].LatestRefURL = row.LatestRefURL
		occurrences[i].LatestAgeDays = row.LatestAgeDays
		occurrences[i].LatestSHA = row.LatestSHA
		if options.ResolveSHA {
			occurrences[i].PinnedUses = pinnedUsesValue(occurrences[i])
		}
	}
}

func workflowLocation(repo Repository, workflowPath string) string {
	if repo.Source == string(ModeLocal) && repo.Path != "" {
		return filepath.Join(repo.Path, filepath.FromSlash(workflowPath))
	}
	if repo.FullName != "" {
		return repo.FullName + "/" + workflowPath
	}
	if repo.Path != "" {
		return filepath.Join(repo.Path, filepath.FromSlash(workflowPath))
	}
	return workflowPath
}

func matchesRemoteRepo(repo Repository, options Options) bool {
	if !matchesRepoFilters(repo, options.RepoFilters) {
		return false
	}
	if !options.IncludeArchived && repo.Archived {
		return false
	}
	if !options.IncludeForks && repo.Fork {
		return false
	}
	if visibility := strings.TrimSpace(options.Visibility); visibility != "" && visibility != "all" && repo.Visibility != visibility {
		return false
	}
	if len(options.Languages) > 0 && !slices.Contains(options.Languages, repo.Language) {
		return false
	}
	return true
}

func matchesRepoFilters(repo Repository, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}

	candidates := []string{
		repo.Name,
		repo.FullName,
		repo.Path,
		filepath.Base(repo.Path),
	}

	for _, pattern := range patterns {
		for _, candidate := range candidates {
			if candidate == "" {
				continue
			}
			matched, err := filepath.Match(pattern, candidate)
			if err == nil && matched {
				return true
			}
		}
	}

	return false
}

func matchesOccurrence(occurrence Occurrence, options Options) bool {
	if !options.IncludeContainers && occurrence.Kind == KindContainer {
		return false
	}
	if options.Kind != "" && options.Kind != KindAll && occurrence.Kind != options.Kind {
		return false
	}
	if len(options.Pinning) > 0 {
		if occurrence.Pinning == "" || !slices.Contains(options.Pinning, occurrence.Pinning) {
			return false
		}
	}
	if options.OnlyFloating && occurrence.Risk != RiskFloating {
		return false
	}
	if match := strings.TrimSpace(options.Match); match != "" {
		target := strings.ToLower(strings.Join([]string{
			occurrence.Name,
			occurrence.Ref,
			occurrence.RepoName,
			occurrence.RepoPathOrFullName,
			occurrence.WorkflowPath,
			occurrence.Job,
			occurrence.Step,
		}, " "))
		if !strings.Contains(target, strings.ToLower(match)) {
			return false
		}
	}
	return true
}

func buildSummary(occurrences []Occurrence) []SummaryRow {
	type summaryKey struct {
		Kind    Kind
		Name    string
		Ref     string
		RefType RefType
		Risk    Risk
		Pinning Pinning
	}

	summary := make(map[summaryKey]*SummaryRow)
	repositories := make(map[summaryKey]map[string]struct{})
	for _, occurrence := range occurrences {
		key := summaryKey{
			Kind:    occurrence.Kind,
			Name:    occurrence.Name,
			Ref:     occurrence.Ref,
			RefType: occurrence.RefType,
			Risk:    occurrence.Risk,
			Pinning: occurrence.Pinning,
		}

		row, ok := summary[key]
		if !ok {
			row = &SummaryRow{
				Kind:    occurrence.Kind,
				Name:    occurrence.Name,
				Ref:     occurrence.Ref,
				RefType: occurrence.RefType,
				Risk:    occurrence.Risk,
				Pinning: occurrence.Pinning,
			}
			summary[key] = row
			repositories[key] = make(map[string]struct{})
		}

		row.OccurrenceCount++
		repositories[key][occurrence.RepoPathOrFullName] = struct{}{}
	}

	rows := make([]SummaryRow, 0, len(summary))
	for key, row := range summary {
		row.RepoCount = len(repositories[key])
		rows = append(rows, *row)
	}

	slices.SortFunc(rows, func(a, b SummaryRow) int {
		if cmp := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); cmp != 0 {
			return cmp
		}
		if cmp := compareSummaryRefs(a, b); cmp != 0 {
			return cmp
		}
		if cmp := strings.Compare(string(a.Kind), string(b.Kind)); cmp != 0 {
			return cmp
		}
		if cmp := strings.Compare(string(a.Pinning), string(b.Pinning)); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.Ref, b.Ref)
	})

	return rows
}

func compareSummaryRefs(a, b SummaryRow) int {
	aRef := strings.TrimSpace(a.Ref)
	bRef := strings.TrimSpace(b.Ref)
	if aRef == bRef {
		return 0
	}

	aVersion, aSpecificity, aSemver := parseSemverTag(aRef)
	bVersion, bSpecificity, bSemver := parseSemverTag(bRef)
	switch {
	case aSemver && bSemver:
		switch {
		case aVersion.LessThan(bVersion):
			return -1
		case bVersion.LessThan(aVersion):
			return 1
		case aSpecificity < bSpecificity:
			return -1
		case bSpecificity < aSpecificity:
			return 1
		}
	case !aSemver && bSemver:
		return -1
	case aSemver && !bSemver:
		return 1
	}

	return strings.Compare(strings.ToLower(aRef), strings.ToLower(bRef))
}
