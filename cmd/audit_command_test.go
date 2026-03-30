package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	colour "github.com/fatih/color"
	"github.com/nickromney-org/github-release-version-checker/internal/audit"
)

func TestRootVersionCompatibility(t *testing.T) {
	SetVersionInfo("1.2.3", "2026-03-30T12:00:00Z", "abc123")

	tests := [][]string{
		{"--version"},
		{"check", "--version"},
	}

	for _, args := range tests {
		resetCommandGlobals()
		output := captureStdout(t, func() {
			cmd := newRootCommand()
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute(%v) error = %v", args, err)
			}
		})

		if !strings.Contains(output, "github-release-version-checker 1.2.3") {
			t.Fatalf("expected version output for args %v, got:\n%s", args, output)
		}
	}
}

func TestAuditWorkflowsLocalJSONAndFailOnFloating(t *testing.T) {
	resetCommandGlobals()
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, ".github", "workflows"), 0o755); err != nil {
		t.Fatalf("mkdir workflows: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, ".github", "workflows", "ci.yml"), []byte("name: ci\njobs:\n  test:\n    steps:\n      - uses: actions/checkout@v4\n      - uses: docker://alpine:latest\n"), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	output, _, err := executeCommandForTest([]string{"audit-workflows", "local", "--path", repoRoot, "--output", "json", "--resolve-latest=false"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(output, "\"occurrences\"") || !strings.Contains(output, "\"floating\"") {
		t.Fatalf("expected JSON output with occurrences and floating risk, got:\n%s", output)
	}

	resetCommandGlobals()
	_, _, err = executeCommandForTest([]string{"audit-workflows", "local", "--path", repoRoot, "--fail-on", "floating", "--resolve-latest=false"})
	if ExitCode(err) != 3 {
		t.Fatalf("ExitCode(err) = %d, want 3 (err=%v)", ExitCode(err), err)
	}
}

func TestAuditWorkflowsLocalSkipsInvalidWorkflow(t *testing.T) {
	resetCommandGlobals()
	workspace := t.TempDir()
	validRepo := filepath.Join(workspace, "backend-api-one")
	invalidRepo := filepath.Join(workspace, "backend-api-two")

	createAuditTestRepo(t, validRepo, "name: ci\njobs:\n  test:\n    steps:\n      - uses: actions/checkout@v4\n")
	createAuditTestRepo(t, invalidRepo, "name: release\njobs:\n  publish:\n    steps:\n      - name: Generate release notes\n        run: |\n          cat > release_notes.md << 'EOF'\n## Installation\n\n```bash\ncurl -fsSL https://example.com/install.sh | bash\n```\nEOF\n")

	output, stderr, err := executeCommandForTest([]string{"audit-workflows", "local", "--path", workspace, "--resolve-latest=false"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(output, "actions/checkout") {
		t.Fatalf("expected table output to include valid workflow findings, got:\n%s", output)
	}
	expectedWarningFile := filepath.Join(invalidRepo, ".github", "workflows", "ci.yml")
	if !strings.Contains(stderr, "warning: skipped "+expectedWarningFile) {
		t.Fatalf("expected stderr warning for invalid workflow, got:\n%s", stderr)
	}

	resetCommandGlobals()
	output, _, err = executeCommandForTest([]string{"audit-workflows", "local", "--path", workspace, "--output", "json", "--resolve-latest=false"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(output, "\"warnings_count\": 1") {
		t.Fatalf("expected JSON output to include one warning, got:\n%s", output)
	}
	if !strings.Contains(output, expectedWarningFile) {
		t.Fatalf("expected JSON output to include warning file path, got:\n%s", output)
	}
}

func TestAuditWorkflowsLocalSingleRepoSummaryOmitsReposColumn(t *testing.T) {
	resetCommandGlobals()
	repoRoot := t.TempDir()
	createAuditTestRepo(t, repoRoot, "name: ci\njobs:\n  test:\n    steps:\n      - uses: actions/checkout@v4\n      - uses: acme/build-action@0123456789abcdef0123456789abcdef01234567\n")

	output, _, err := executeCommandForTest([]string{"audit-workflows", "local", "--path", repoRoot, "--resolve-latest=false"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if strings.Contains(output, "REPOS") {
		t.Fatalf("expected single-repo summary output to omit REPOS column, got:\n%s", output)
	}
	if !strings.Contains(output, "PINNING") {
		t.Fatalf("expected summary output to include PINNING column, got:\n%s", output)
	}
}

func TestAuditWorkflowsLocalPinningFilter(t *testing.T) {
	resetCommandGlobals()
	repoRoot := t.TempDir()
	createAuditTestRepo(t, repoRoot, "name: ci\njobs:\n  test:\n    steps:\n      - uses: actions/checkout@main\n      - uses: actions/setup-go@v5\n      - uses: acme/build-action@0123456789abcdef0123456789abcdef01234567\n")

	output, _, err := executeCommandForTest([]string{"audit-workflows", "local", "--path", repoRoot, "--output", "json", "--pinning", "sha", "--resolve-latest=false"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(output, "\"pinning\": \"sha\"") {
		t.Fatalf("expected JSON output to include sha pinning, got:\n%s", output)
	}
	if !strings.Contains(output, "acme/build-action") {
		t.Fatalf("expected JSON output to include SHA-pinned action, got:\n%s", output)
	}
	if strings.Contains(output, "actions/setup-go") || strings.Contains(output, "actions/checkout") {
		t.Fatalf("expected JSON output to exclude non-SHA matches, got:\n%s", output)
	}
}

func TestAuditWorkflowsExcludesContainersByDefault(t *testing.T) {
	resetCommandGlobals()
	repoRoot := t.TempDir()
	createAuditTestRepo(t, repoRoot, "name: ci\njobs:\n  test:\n    container:\n      image: mysql:8.0\n    steps:\n      - uses: actions/checkout@v4\n")

	output, _, err := executeCommandForTest([]string{"audit-workflows", "local", "--path", repoRoot, "--output", "json", "--resolve-latest=false"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if strings.Contains(output, "\"kind\": \"container\"") || strings.Contains(output, "\"name\": \"mysql\"") {
		t.Fatalf("expected workflow audit to exclude containers, got:\n%s", output)
	}
	if !strings.Contains(output, "\"name\": \"actions/checkout\"") {
		t.Fatalf("expected workflow audit to keep action usages, got:\n%s", output)
	}
}

func TestAuditContainersFindsContainers(t *testing.T) {
	resetCommandGlobals()
	repoRoot := t.TempDir()
	createAuditTestRepo(t, repoRoot, "name: ci\njobs:\n  test:\n    container:\n      image: mysql:8.0\n    services:\n      redis:\n        image: redis:7.2.4\n    steps:\n      - uses: actions/checkout@v4\n")

	output, _, err := executeCommandForTest([]string{"audit-containers", "local", "--path", repoRoot, "--output", "json", "--resolve-latest=false"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(output, "\"kind\": \"container\"") || !strings.Contains(output, "\"name\": \"mysql\"") {
		t.Fatalf("expected container audit to include job container, got:\n%s", output)
	}
	if !strings.Contains(output, "\"name\": \"redis\"") {
		t.Fatalf("expected container audit to include service container, got:\n%s", output)
	}
	if strings.Contains(output, "\"name\": \"actions/checkout\"") {
		t.Fatalf("expected container audit to exclude workflow actions, got:\n%s", output)
	}
}

func TestAuditCommandOptionsPinSHAImpliesResolveLatest(t *testing.T) {
	options := &auditCommandOptions{
		format:       "table",
		view:         "summary",
		kind:         "all",
		failOn:       "none",
		cooldownDays: 7,
		pinSHA:       true,
	}

	auditOptions, err := options.toAuditOptions(audit.ModeLocal)
	if err != nil {
		t.Fatalf("toAuditOptions() error = %v", err)
	}

	if !auditOptions.ResolveLatest {
		t.Fatalf("ResolveLatest = false, want true when pinSHA is enabled")
	}
	if !auditOptions.ResolveSHA {
		t.Fatalf("ResolveSHA = false, want true when pinSHA is enabled")
	}
	if auditOptions.CooldownDays != 7 {
		t.Fatalf("CooldownDays = %d, want 7", auditOptions.CooldownDays)
	}
}

func TestAuditContainersHelpOmitsPinSHAFlag(t *testing.T) {
	resetCommandGlobals()
	output, _, err := executeCommandForTest([]string{"audit-containers", "local", "--help"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if strings.Contains(output, "--pin-sha") {
		t.Fatalf("expected audit-containers help to omit --pin-sha, got:\n%s", output)
	}
}

func TestAuditCommandOptionsCooldownValidation(t *testing.T) {
	options := &auditCommandOptions{
		format:       "table",
		view:         "summary",
		kind:         "all",
		failOn:       "none",
		cooldownDays: -1,
	}

	_, err := options.toAuditOptions(audit.ModeLocal)
	if err == nil || !strings.Contains(err.Error(), "--cooldown must be 0 or greater") {
		t.Fatalf("expected cooldown validation error, got %v", err)
	}
}

func TestAuditOrgAcceptsPositionalArgument(t *testing.T) {
	options := &auditCommandOptions{
		format:       "table",
		view:         "summary",
		kind:         "all",
		failOn:       "none",
		cooldownDays: 7,
	}

	if err := applyAuditOrgArgs(options, []string{"nickromney-org"}); err != nil {
		t.Fatalf("applyAuditOrgArgs() error = %v", err)
	}
	if options.org != "nickromney-org" {
		t.Fatalf("org = %q, want %q", options.org, "nickromney-org")
	}
}

func TestAuditRepoAcceptsPositionalArgument(t *testing.T) {
	options := &auditCommandOptions{
		format:       "table",
		view:         "summary",
		kind:         "all",
		failOn:       "none",
		cooldownDays: 7,
	}

	if err := applyAuditRepoArgs(options, []string{"acme/backend-api"}); err != nil {
		t.Fatalf("applyAuditRepoArgs() error = %v", err)
	}
	if options.repo != "acme/backend-api" {
		t.Fatalf("repo = %q, want %q", options.repo, "acme/backend-api")
	}
}

func TestAuditOwnerAcceptsPositionalArgument(t *testing.T) {
	options := &auditCommandOptions{
		format:       "table",
		view:         "summary",
		kind:         "all",
		failOn:       "none",
		cooldownDays: 7,
	}

	if err := applyAuditOwnerArgs(options, []string{"nickromney"}); err != nil {
		t.Fatalf("applyAuditOwnerArgs() error = %v", err)
	}
	if options.owner != "nickromney" {
		t.Fatalf("owner = %q, want %q", options.owner, "nickromney")
	}
}

func TestAuditSelectorArgRejectsConflictingValues(t *testing.T) {
	options := &auditCommandOptions{
		org: "from-flag",
	}

	err := applyAuditOrgArgs(options, []string{"from-arg"})
	if err == nil || !strings.Contains(err.Error(), "specified twice") {
		t.Fatalf("expected conflict error, got %v", err)
	}
}

func TestAuditOwnerHelpShowsPositionalUsage(t *testing.T) {
	resetCommandGlobals()
	output, _, err := executeCommandForTest([]string{"audit-workflows", "owner", "--help"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(output, "audit-workflows owner [owner]") {
		t.Fatalf("expected owner help usage, got:\n%s", output)
	}
}

func TestAuditWorkflowsParentHelpShowsDiscoverableExamples(t *testing.T) {
	resetCommandGlobals()
	output, _, err := executeCommandForTest([]string{"audit-workflows", "--help"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(output, "audit-workflows local --path ~/src --view occurrences -o markdown") {
		t.Fatalf("expected parent help to include occurrences markdown example, got:\n%s", output)
	}
	if !strings.Contains(output, "audit-workflows owner my-org --repo-filter 'backend-api-*' -o json") {
		t.Fatalf("expected parent help to include owner json example, got:\n%s", output)
	}
}

func TestAuditContainersParentHelpOmitsPinSHAExample(t *testing.T) {
	resetCommandGlobals()
	output, _, err := executeCommandForTest([]string{"audit-containers", "--help"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if strings.Contains(output, "--pin-sha") {
		t.Fatalf("expected container parent help to omit pin-sha, got:\n%s", output)
	}
	if !strings.Contains(output, "audit-containers local --path ~/src --view occurrences -o markdown") {
		t.Fatalf("expected container parent help to include markdown example, got:\n%s", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	oldColourOutput := colour.Output
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = writer
	colour.Output = writer
	defer func() {
		os.Stdout = oldStdout
		colour.Output = oldColourOutput
	}()

	fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close() error = %v", err)
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	return string(data)
}

func executeCommandForTest(args []string) (string, string, error) {
	cmd := newRootCommand()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func createAuditTestRepo(t *testing.T, root, workflow string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".github", "workflows"), 0o755); err != nil {
		t.Fatalf("mkdir workflows: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".github", "workflows", "ci.yml"), []byte(workflow), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
}

func resetCommandGlobals() {
	comparisonVersion = ""
	criticalAgeDays = 12
	maxAgeDays = 30
	verbose = false
	jsonOutput = false
	ciOutput = false
	quiet = false
	githubToken = ""
	showVersion = false
	noCache = false
	repository = ""
	cachePath = ""
	policyType = ""
	maxVersions = 3
}
