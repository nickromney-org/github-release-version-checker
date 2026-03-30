package audit

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

type fakeRemoteSource struct {
	repositories []Repository
	workflows    map[string][]WorkflowFile
}

type fakeLatestResolver struct {
	values map[string]LatestVersion
	shas   map[string]string
}

func (f fakeLatestResolver) ResolveLatest(_ context.Context, kind Kind, name string, _ LatestResolveOptions) (LatestVersion, error) {
	return f.values[string(kind)+":"+name], nil
}

func (f fakeLatestResolver) ResolveSHA(_ context.Context, kind Kind, name, ref string) (string, error) {
	return f.shas[string(kind)+":"+name+"@"+ref], nil
}

func (f *fakeRemoteSource) GetRepository(_ context.Context, owner, repo string) (Repository, error) {
	fullName := owner + "/" + repo
	for _, repository := range f.repositories {
		if repository.FullName == fullName {
			return repository, nil
		}
	}
	return Repository{}, nil
}

func (f *fakeRemoteSource) ListRepositoriesByOrg(_ context.Context, _ string) ([]Repository, error) {
	return f.repositories, nil
}

func (f *fakeRemoteSource) ListRepositoriesByOwner(_ context.Context, _ string, _ string) ([]Repository, error) {
	return f.repositories, nil
}

func (f *fakeRemoteSource) ListWorkflowFiles(_ context.Context, repo Repository) ([]WorkflowFile, error) {
	return f.workflows[repo.FullName], nil
}

func TestScannerScanOrgAppliesRemoteFilters(t *testing.T) {
	remote := &fakeRemoteSource{
		repositories: []Repository{
			{Name: "backend-api-one", FullName: "acme/backend-api-one", Visibility: "private", Language: "Go", Source: "remote"},
			{Name: "backend-api-two", FullName: "acme/backend-api-two", Visibility: "private", Language: "Go", Archived: true, Source: "remote"},
			{Name: "frontend-web", FullName: "acme/frontend-web", Visibility: "private", Language: "TypeScript", Source: "remote"},
		},
		workflows: map[string][]WorkflowFile{
			"acme/backend-api-one": {{
				Path:    ".github/workflows/ci.yml",
				Content: []byte("name: ci\njobs:\n  test:\n    steps:\n      - uses: actions/checkout@v4\n      - uses: docker://alpine:latest\n"),
			}},
			"acme/backend-api-two": {{
				Path:    ".github/workflows/ci.yml",
				Content: []byte("name: ci\njobs:\n  test:\n    steps:\n      - uses: actions/cache@v4\n"),
			}},
		},
	}

	scanner := NewScanner(remote)
	result, err := scanner.ScanOrg(context.Background(), Options{
		Mode:            ModeOrg,
		Org:             "acme",
		Host:            "github.example.com",
		RepoFilters:     []string{"backend-api-*"},
		Visibility:      "private",
		Languages:       []string{"Go"},
		IncludeArchived: false,
		Kind:            KindAll,
		View:            ViewSummary,
		Format:          FormatJSON,
		FailOn:          FailOnNone,
	})
	if err != nil {
		t.Fatalf("ScanOrg() error = %v", err)
	}

	if result.Metadata.RepositoriesScanned != 1 {
		t.Fatalf("RepositoriesScanned = %d, want 1", result.Metadata.RepositoriesScanned)
	}
	if len(result.Occurrences) != 2 {
		t.Fatalf("got %d occurrences, want 2", len(result.Occurrences))
	}
	if result.Occurrences[0].RepoPathOrFullName != "acme/backend-api-one" {
		t.Fatalf("RepoPathOrFullName = %q, want acme/backend-api-one", result.Occurrences[0].RepoPathOrFullName)
	}
}

func TestScannerScanOwnerAppliesRemoteFilters(t *testing.T) {
	remote := &fakeRemoteSource{
		repositories: []Repository{
			{Name: "backend-api-one", FullName: "nick/backend-api-one", Visibility: "private", Language: "Go", Source: "remote"},
			{Name: "frontend-web", FullName: "nick/frontend-web", Visibility: "public", Language: "TypeScript", Source: "remote"},
		},
		workflows: map[string][]WorkflowFile{
			"nick/backend-api-one": {{
				Path:    ".github/workflows/ci.yml",
				Content: []byte("name: ci\njobs:\n  test:\n    steps:\n      - uses: actions/checkout@v4\n"),
			}},
			"nick/frontend-web": {{
				Path:    ".github/workflows/ci.yml",
				Content: []byte("name: ci\njobs:\n  test:\n    steps:\n      - uses: actions/cache@v4\n"),
			}},
		},
	}

	scanner := NewScanner(remote)
	result, err := scanner.ScanOwner(context.Background(), Options{
		Mode:        ModeOwner,
		Owner:       "nick",
		Visibility:  "private",
		RepoFilters: []string{"backend-*"},
		Kind:        KindAll,
		View:        ViewSummary,
		Format:      FormatJSON,
		FailOn:      FailOnNone,
	})
	if err != nil {
		t.Fatalf("ScanOwner() error = %v", err)
	}

	if result.Metadata.Owner != "nick" {
		t.Fatalf("Owner = %q, want nick", result.Metadata.Owner)
	}
	if result.Metadata.RepositoriesScanned != 1 {
		t.Fatalf("RepositoriesScanned = %d, want 1", result.Metadata.RepositoriesScanned)
	}
	if len(result.Occurrences) != 1 {
		t.Fatalf("got %d occurrences, want 1", len(result.Occurrences))
	}
	if result.Occurrences[0].RepoPathOrFullName != "nick/backend-api-one" {
		t.Fatalf("RepoPathOrFullName = %q, want nick/backend-api-one", result.Occurrences[0].RepoPathOrFullName)
	}
}

func TestScannerPopulatesLatestRefs(t *testing.T) {
	root := t.TempDir()
	repoPath := filepath.Join(root, "backend-api-one")

	createLocalRepo(t, repoPath, "git@github.com:acme/backend-api-one.git", "name: ci\njobs:\n  test:\n    steps:\n      - uses: actions/checkout@v6\n      - uses: actions/setup-go@v5\n")

	scanner := NewScanner(nil)
	scanner.Latest = fakeLatestResolver{values: map[string]LatestVersion{
		"action:actions/checkout": {
			Ref:     "v6.0.1",
			AgeDays: intPtr(10),
		},
		"action:actions/setup-go": {
			Ref:     "v5.3.0",
			AgeDays: intPtr(12),
		},
	}, shas: map[string]string{
		"action:actions/checkout@v6.0.1": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"action:actions/setup-go@v5.3.0": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}}

	result, err := scanner.ScanLocal(context.Background(), Options{
		Mode:          ModeLocal,
		Path:          root,
		MaxDepth:      2,
		Kind:          KindAll,
		ResolveLatest: true,
		ResolveSHA:    true,
		View:          ViewSummary,
		Format:        FormatJSON,
		FailOn:        FailOnNone,
	})
	if err != nil {
		t.Fatalf("ScanLocal() error = %v", err)
	}

	wantLatest := map[string]struct {
		ref       string
		sha       string
		age       int
		upstream  string
		latestURL string
	}{
		"actions/checkout": {ref: "v6.0.1", sha: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", age: 10, upstream: "https://github.com/actions/checkout", latestURL: "https://github.com/actions/checkout/tree/v6.0.1"},
		"actions/setup-go": {ref: "v5.3.0", sha: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", age: 12, upstream: "https://github.com/actions/setup-go", latestURL: "https://github.com/actions/setup-go/tree/v5.3.0"},
	}
	for _, row := range result.Summary {
		want := wantLatest[row.Name]
		if row.LatestRef != want.ref {
			t.Fatalf("LatestRef for %s = %q, want %q", row.Name, row.LatestRef, want.ref)
		}
		if row.LatestSHA != want.sha {
			t.Fatalf("LatestSHA for %s = %q, want %q", row.Name, row.LatestSHA, want.sha)
		}
		if row.LatestAgeDays == nil || *row.LatestAgeDays != want.age {
			t.Fatalf("LatestAgeDays for %s = %v, want %d", row.Name, row.LatestAgeDays, want.age)
		}
		if row.UpstreamURL != want.upstream {
			t.Fatalf("UpstreamURL for %s = %q, want %q", row.Name, row.UpstreamURL, want.upstream)
		}
		if row.LatestRefURL != want.latestURL {
			t.Fatalf("LatestRefURL for %s = %q, want %q", row.Name, row.LatestRefURL, want.latestURL)
		}
	}

	if len(result.Occurrences) != 2 {
		t.Fatalf("len(Occurrences) = %d, want 2", len(result.Occurrences))
	}
	for _, occurrence := range result.Occurrences {
		want := wantLatest[occurrence.Name]
		if occurrence.LatestRef != want.ref {
			t.Fatalf("Occurrence LatestRef for %s = %q, want %q", occurrence.Name, occurrence.LatestRef, want.ref)
		}
		if occurrence.LatestSHA != want.sha {
			t.Fatalf("Occurrence LatestSHA for %s = %q, want %q", occurrence.Name, occurrence.LatestSHA, want.sha)
		}
		if occurrence.LatestAgeDays == nil || *occurrence.LatestAgeDays != want.age {
			t.Fatalf("Occurrence LatestAgeDays for %s = %v, want %d", occurrence.Name, occurrence.LatestAgeDays, want.age)
		}
		if occurrence.UpstreamURL != want.upstream {
			t.Fatalf("Occurrence UpstreamURL for %s = %q, want %q", occurrence.Name, occurrence.UpstreamURL, want.upstream)
		}
		if occurrence.LatestRefURL != want.latestURL {
			t.Fatalf("Occurrence LatestRefURL for %s = %q, want %q", occurrence.Name, occurrence.LatestRefURL, want.latestURL)
		}
		if occurrence.PinnedUses == "" {
			t.Fatalf("Occurrence PinnedUses for %s is empty", occurrence.Name)
		}
	}
}

func TestRenderSummaryAndOccurrences(t *testing.T) {
	result := &Result{
		Metadata: Metadata{Mode: ModeLocal, RepositoriesScanned: 2},
		Summary: []SummaryRow{{
			Kind:            KindAction,
			Name:            "actions/checkout",
			UpstreamURL:     "https://github.com/actions/checkout",
			Ref:             "v4",
			LatestRef:       "v4.2.2",
			LatestRefURL:    "https://github.com/actions/checkout/tree/v4.2.2",
			LatestAgeDays:   intPtr(14),
			LatestSHA:       "db41740e12847bb616a339b75eb9414e711417df",
			RefType:         RefTypeMajor,
			Risk:            RiskReview,
			Pinning:         PinningSemver,
			RepoCount:       2,
			OccurrenceCount: 3,
		}},
		Occurrences: []Occurrence{{
			Kind:               KindAction,
			Name:               "actions/checkout",
			UpstreamURL:        "https://github.com/actions/checkout",
			Ref:                "v4",
			LatestRef:          "v4.2.2",
			LatestRefURL:       "https://github.com/actions/checkout/tree/v4.2.2",
			LatestAgeDays:      intPtr(14),
			LatestSHA:          "db41740e12847bb616a339b75eb9414e711417df",
			PinnedUses:         "actions/checkout@db41740e12847bb616a339b75eb9414e711417df # pin@v4.2.2",
			RefType:            RefTypeMajor,
			Risk:               RiskReview,
			Pinning:            PinningSemver,
			RepoName:           "backend-api-one",
			RepoFullName:       "acme/backend-api-one",
			RepoPath:           "/tmp/backend-api-one",
			RepoDefaultBranch:  "main",
			RepoPathOrFullName: "acme/backend-api-one",
			WorkflowPath:       ".github/workflows/ci.yml",
			Job:                "test",
			Step:               "Checkout",
			Line:               8,
		}},
	}

	var table bytes.Buffer
	if err := Render(&table, result, FormatTable, ViewSummary); err != nil {
		t.Fatalf("Render(table, summary) error = %v", err)
	}
	if !strings.Contains(table.String(), "actions/checkout") || !strings.Contains(table.String(), "v4.2.2") {
		t.Fatalf("expected summary table to contain dependency, got:\n%s", table.String())
	}
	if !strings.Contains(table.String(), "db41740e128") {
		t.Fatalf("expected summary table to include shortened latest SHA, got:\n%s", table.String())
	}
	if !strings.Contains(table.String(), "14") {
		t.Fatalf("expected summary table to include latest age, got:\n%s", table.String())
	}

	var csv bytes.Buffer
	if err := Render(&csv, result, FormatCSV, ViewOccurrences); err != nil {
		t.Fatalf("Render(csv, occurrences) error = %v", err)
	}
	if !strings.Contains(csv.String(), "repo,workflow_path,job,step,kind,name,current_ref,latest_ref,latest_age_days,latest_sha,pinning,pinned_uses,line") {
		t.Fatalf("expected CSV header, got:\n%s", csv.String())
	}
	if !strings.Contains(csv.String(), "v4.2.2") {
		t.Fatalf("expected CSV output to include latest ref, got:\n%s", csv.String())
	}
	if !strings.Contains(csv.String(), ",14,db41740e12847bb616a339b75eb9414e711417df,") {
		t.Fatalf("expected CSV output to include latest age and SHA, got:\n%s", csv.String())
	}
	if !strings.Contains(csv.String(), "actions/checkout@db41740e12847bb616a339b75eb9414e711417df # pin@v4.2.2") {
		t.Fatalf("expected CSV output to include pinned uses value, got:\n%s", csv.String())
	}

	var json bytes.Buffer
	if err := Render(&json, result, FormatJSON, ViewSummary); err != nil {
		t.Fatalf("Render(json, summary) error = %v", err)
	}
	if !strings.Contains(json.String(), "\"summary\"") || !strings.Contains(json.String(), "\"occurrences\"") {
		t.Fatalf("expected JSON output to include summary and occurrences, got:\n%s", json.String())
	}
	if !strings.Contains(json.String(), "\"upstream_url\": \"https://github.com/actions/checkout\"") {
		t.Fatalf("expected JSON output to include upstream url, got:\n%s", json.String())
	}
	if !strings.Contains(json.String(), "\"latest_ref_url\": \"https://github.com/actions/checkout/tree/v4.2.2\"") {
		t.Fatalf("expected JSON output to include latest ref url, got:\n%s", json.String())
	}

	var markdown bytes.Buffer
	if err := Render(&markdown, result, FormatMarkdown, ViewOccurrences); err != nil {
		t.Fatalf("Render(markdown, occurrences) error = %v", err)
	}
	if !strings.Contains(markdown.String(), "[acme/backend-api-one](/tmp/backend-api-one)") {
		t.Fatalf("expected Markdown repo link, got:\n%s", markdown.String())
	}
	if !strings.Contains(markdown.String(), "[/tmp/backend-api-one/.github/workflows/ci.yml](/tmp/backend-api-one/.github/workflows/ci.yml)") {
		t.Fatalf("expected Markdown workflow link, got:\n%s", markdown.String())
	}
	if !strings.Contains(markdown.String(), "[actions/checkout](https://github.com/actions/checkout)") {
		t.Fatalf("expected Markdown output to include upstream link, got:\n%s", markdown.String())
	}
	if !strings.Contains(markdown.String(), "[v4.2.2](https://github.com/actions/checkout/tree/v4.2.2)") {
		t.Fatalf("expected Markdown output to include latest ref link, got:\n%s", markdown.String())
	}
	if !strings.Contains(markdown.String(), "| 14 | `db41740e12847bb616a339b75eb9414e711417df` |") {
		t.Fatalf("expected Markdown output to include latest age and SHA, got:\n%s", markdown.String())
	}
	if !strings.Contains(markdown.String(), "`actions/checkout@db41740e12847bb616a339b75eb9414e711417df # pin@v4.2.2`") {
		t.Fatalf("expected Markdown output to include pinned uses value, got:\n%s", markdown.String())
	}
}

func TestBuildSummarySortsByNameThenCurrentRef(t *testing.T) {
	rows := buildSummary([]Occurrence{
		{Kind: KindAction, Name: "ludeeus/action-shellcheck", Ref: "master", RefType: RefTypeLatest, Risk: RiskFloating, Pinning: PinningFloating, RepoPathOrFullName: "one"},
		{Kind: KindAction, Name: "actions/cache", Ref: "v4", RefType: RefTypeMajor, Risk: RiskReview, Pinning: PinningSemver, RepoPathOrFullName: "one"},
		{Kind: KindAction, Name: "ASzc/change-string-case-action", Ref: "v6", RefType: RefTypeMajor, Risk: RiskReview, Pinning: PinningSemver, RepoPathOrFullName: "one"},
		{Kind: KindAction, Name: "actions/checkout", Ref: "v6", RefType: RefTypeMajor, Risk: RiskReview, Pinning: PinningSemver, RepoPathOrFullName: "one"},
		{Kind: KindAction, Name: "actions/checkout", Ref: "master", RefType: RefTypeLatest, Risk: RiskFloating, Pinning: PinningFloating, RepoPathOrFullName: "one"},
		{Kind: KindAction, Name: "actions/checkout", Ref: "v6.0.2", RefType: RefTypeExact, Risk: RiskReview, Pinning: PinningSemver, RepoPathOrFullName: "one"},
		{Kind: KindAction, Name: "actions/checkout", Ref: "v3", RefType: RefTypeMajor, Risk: RiskReview, Pinning: PinningSemver, RepoPathOrFullName: "one"},
	})

	got := make([]string, 0, len(rows))
	for _, row := range rows {
		got = append(got, row.Name+"@"+row.Ref)
	}

	want := []string{
		"actions/cache@v4",
		"actions/checkout@master",
		"actions/checkout@v3",
		"actions/checkout@v6",
		"actions/checkout@v6.0.2",
		"ASzc/change-string-case-action@v6",
		"ludeeus/action-shellcheck@master",
	}

	if len(got) != len(want) {
		t.Fatalf("got %d rows, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("row %d = %q, want %q (full order: %v)", i, got[i], want[i], got)
		}
	}
}
