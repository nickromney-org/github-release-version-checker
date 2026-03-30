package audit

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverLocalRepositoriesWorkspaceAndDepth(t *testing.T) {
	root := t.TempDir()

	repoA := filepath.Join(root, "backend-api-one")
	repoB := filepath.Join(root, "services", "backend-api-two")
	repoTooDeep := filepath.Join(root, "nested", "one", "two", "backend-api-three")

	createLocalRepo(t, repoA, "git@github.com:acme/backend-api-one.git", "name: ci\njobs:\n  test:\n    steps:\n      - uses: actions/checkout@v4\n")
	createLocalRepo(t, repoB, "https://github.com/acme/backend-api-two.git", "name: ci\njobs:\n  test:\n    steps:\n      - uses: actions/setup-go@v5\n")
	createLocalRepo(t, repoTooDeep, "", "name: ci\njobs:\n  test:\n    steps:\n      - uses: actions/cache@v4\n")

	repositories, err := DiscoverLocalRepositories(root, 3)
	if err != nil {
		t.Fatalf("DiscoverLocalRepositories() error = %v", err)
	}

	if len(repositories) != 2 {
		t.Fatalf("got %d repositories, want 2", len(repositories))
	}
	if repositories[0].FullName != "acme/backend-api-one" && repositories[1].FullName != "acme/backend-api-one" {
		t.Fatalf("expected acme/backend-api-one in discovered repositories: %#v", repositories)
	}
}

func TestDiscoverLocalRepositoriesWorktreeGitFile(t *testing.T) {
	root := t.TempDir()
	repoPath := filepath.Join(root, "backend-api")
	gitCommon := filepath.Join(root, "git-common")
	worktreeGitDir := filepath.Join(root, "git-worktrees", "backend-api")

	if err := os.MkdirAll(filepath.Join(repoPath, ".github", "workflows"), 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	if err := os.MkdirAll(worktreeGitDir, 0o755); err != nil {
		t.Fatalf("mkdir worktree git dir: %v", err)
	}
	if err := os.MkdirAll(gitCommon, 0o755); err != nil {
		t.Fatalf("mkdir common git dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, ".git"), []byte("gitdir: "+worktreeGitDir+"\n"), 0o644); err != nil {
		t.Fatalf("write .git file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreeGitDir, "commondir"), []byte(gitCommon+"\n"), 0o644); err != nil {
		t.Fatalf("write commondir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitCommon, "config"), []byte("[remote \"origin\"]\n\turl = git@github.com:acme/backend-api.git\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, ".github", "workflows", "ci.yml"), []byte("name: ci\njobs:\n  test:\n    steps:\n      - uses: actions/checkout@v4\n"), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	repositories, err := DiscoverLocalRepositories(repoPath, 3)
	if err != nil {
		t.Fatalf("DiscoverLocalRepositories() error = %v", err)
	}
	if len(repositories) != 1 {
		t.Fatalf("got %d repositories, want 1", len(repositories))
	}
	if repositories[0].FullName != "acme/backend-api" {
		t.Fatalf("FullName = %q, want acme/backend-api", repositories[0].FullName)
	}
}

func TestScannerScanLocalRepoFilterAndNoWorkflows(t *testing.T) {
	root := t.TempDir()
	matchingRepo := filepath.Join(root, "backend-api-one")
	emptyRepo := filepath.Join(root, "frontend-web")

	createLocalRepo(t, matchingRepo, "git@github.com:acme/backend-api-one.git", "name: ci\njobs:\n  test:\n    steps:\n      - uses: actions/checkout@v4\n      - uses: docker://alpine:latest\n")
	createLocalRepo(t, emptyRepo, "", "")

	scanner := NewScanner(nil)
	result, err := scanner.ScanLocal(context.Background(), Options{
		Mode:        ModeLocal,
		Path:        root,
		MaxDepth:    2,
		RepoFilters: []string{"backend-api-*"},
		Kind:        KindAll,
		View:        ViewSummary,
		Format:      FormatJSON,
		FailOn:      FailOnNone,
	})
	if err != nil {
		t.Fatalf("ScanLocal() error = %v", err)
	}

	if result.Metadata.RepositoriesScanned != 1 {
		t.Fatalf("RepositoriesScanned = %d, want 1", result.Metadata.RepositoriesScanned)
	}
	if result.Metadata.RepositoriesWithFlows != 1 {
		t.Fatalf("RepositoriesWithFlows = %d, want 1", result.Metadata.RepositoriesWithFlows)
	}
	if len(result.Occurrences) != 2 {
		t.Fatalf("got %d occurrences, want 2", len(result.Occurrences))
	}
	if !result.HasFloating() {
		t.Fatal("expected floating occurrence from docker://alpine:latest")
	}
}

func TestScannerScanLocalSkipsInvalidWorkflowWithWarning(t *testing.T) {
	root := t.TempDir()
	validRepo := filepath.Join(root, "backend-api-one")
	invalidRepo := filepath.Join(root, "backend-api-two")

	createLocalRepo(t, validRepo, "git@github.com:acme/backend-api-one.git", "name: ci\njobs:\n  test:\n    steps:\n      - uses: actions/checkout@v4\n")
	createLocalRepo(t, invalidRepo, "git@github.com:acme/backend-api-two.git", "name: release\njobs:\n  publish:\n    runs-on: ubuntu-latest\n    steps:\n      - name: Generate release notes\n        run: |\n          cat > release_notes.md << 'EOF'\n## Installation\n\n```bash\ncurl -fsSL https://example.com/install.sh | bash\n```\nEOF\n")

	scanner := NewScanner(nil)
	result, err := scanner.ScanLocal(context.Background(), Options{
		Mode:     ModeLocal,
		Path:     root,
		MaxDepth: 2,
		Kind:     KindAll,
		View:     ViewSummary,
		Format:   FormatJSON,
		FailOn:   FailOnNone,
	})
	if err != nil {
		t.Fatalf("ScanLocal() error = %v", err)
	}

	if len(result.Occurrences) != 1 {
		t.Fatalf("got %d occurrences, want 1", len(result.Occurrences))
	}
	if result.Metadata.WarningsCount != 1 {
		t.Fatalf("WarningsCount = %d, want 1", result.Metadata.WarningsCount)
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("got %d warnings, want 1", len(result.Warnings))
	}
	if result.Warnings[0].File != filepath.Join(invalidRepo, ".github", "workflows", "ci.yml") {
		t.Fatalf("warning file = %q, want %q", result.Warnings[0].File, filepath.Join(invalidRepo, ".github", "workflows", "ci.yml"))
	}
}

func TestScannerScanLocalPinningFilters(t *testing.T) {
	root := t.TempDir()
	repoPath := filepath.Join(root, "backend-api-one")

	createLocalRepo(t, repoPath, "git@github.com:acme/backend-api-one.git", "name: ci\njobs:\n  test:\n    container:\n      image: redis:7.2.4\n    steps:\n      - uses: actions/checkout@main\n      - uses: actions/setup-go@v5\n      - uses: acme/build-action@0123456789abcdef0123456789abcdef01234567\n")

	tests := []struct {
		name      string
		pinning   []Pinning
		wantNames []string
		wantCount int
	}{
		{
			name:      "floating",
			pinning:   []Pinning{PinningFloating},
			wantNames: []string{"actions/checkout"},
			wantCount: 1,
		},
		{
			name:      "semver",
			pinning:   []Pinning{PinningSemver},
			wantNames: []string{"actions/setup-go", "redis"},
			wantCount: 2,
		},
		{
			name:      "sha",
			pinning:   []Pinning{PinningSHA},
			wantNames: []string{"acme/build-action"},
			wantCount: 1,
		},
	}

	scanner := NewScanner(nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := scanner.ScanLocal(context.Background(), Options{
				Mode:              ModeLocal,
				Path:              root,
				MaxDepth:          2,
				Kind:              KindAll,
				Pinning:           tt.pinning,
				IncludeContainers: true,
				View:              ViewSummary,
				Format:            FormatJSON,
				FailOn:            FailOnNone,
			})
			if err != nil {
				t.Fatalf("ScanLocal() error = %v", err)
			}
			if len(result.Occurrences) != tt.wantCount {
				t.Fatalf("got %d occurrences, want %d", len(result.Occurrences), tt.wantCount)
			}
			for _, wantName := range tt.wantNames {
				found := false
				for _, occurrence := range result.Occurrences {
					if occurrence.Name == wantName {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("expected occurrence %q in %#v", wantName, result.Occurrences)
				}
			}
		})
	}
}

func createLocalRepo(t *testing.T, root, remoteURL, workflow string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if remoteURL != "" {
		config := "[remote \"origin\"]\n\turl = " + remoteURL + "\n"
		if err := os.WriteFile(filepath.Join(root, ".git", "config"), []byte(config), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}
	}
	if workflow == "" {
		return
	}
	if err := os.MkdirAll(filepath.Join(root, ".github", "workflows"), 0o755); err != nil {
		t.Fatalf("mkdir workflows: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".github", "workflows", "ci.yml"), []byte(workflow), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
}
