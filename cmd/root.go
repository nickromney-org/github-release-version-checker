package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	colour "github.com/fatih/color"
	"github.com/nickromney-org/github-release-version-checker/internal/cache"
	"github.com/nickromney-org/github-release-version-checker/internal/config"
	"github.com/nickromney-org/github-release-version-checker/internal/policy"
	"github.com/nickromney-org/github-release-version-checker/pkg/checker"
	"github.com/nickromney-org/github-release-version-checker/pkg/client"
	"github.com/spf13/cobra"
)

var (
	comparisonVersion string
	criticalAgeDays   int
	maxAgeDays        int
	verbose           bool
	jsonOutput        bool
	ciOutput          bool
	quiet             bool
	githubToken       string
	showVersion       bool
	noCache           bool

	// New flags for multi-repository support
	repository  string
	cachePath   string
	policyType  string
	maxVersions int

	// Version information (set via SetVersionInfo from main)
	appVersion = "dev"
	buildTime  = "unknown"
	gitCommit  = "unknown"

	// Colours for output
	green  = colour.New(colour.FgGreen, colour.Bold)
	yellow = colour.New(colour.FgYellow, colour.Bold)
	red    = colour.New(colour.FgRed, colour.Bold)
	cyan   = colour.New(colour.FgCyan)
	grey   = colour.New(colour.FgHiBlack) // Faint grey for timestamps
)

const (
	rootShort = "Check GitHub release version status against expiry policies"
	rootLong  = `Check if versions are up to date against configurable expiry policies.

Supports multiple repositories with both time-based (days) and version-based
(semantic versioning) policies. Defaults to GitHub Actions runner with 30-day policy.`
	rootExample = `  # Check latest GitHub Actions runner version (default, days-based policy)
  github-release-version-checker
  github-release-version-checker -c 2.328.0

  # Explicit check command
  github-release-version-checker check --repo k8s -c 1.28.0

  # Kubernetes (version-based: 3 minor versions supported)
  github-release-version-checker --repo kubernetes/kubernetes -c 1.31.12
  github-release-version-checker --repo k8s -c 1.28.0

  # Node.js (version-based: 3 major versions supported)
  github-release-version-checker --repo nodejs/node -c v20.0.0
  github-release-version-checker --repo node -c v18.20.0

  # Pulumi (version-based policy)
  github-release-version-checker --repo pulumi/pulumi -c 3.204.0

  # HashiCorp Terraform (version-based policy)
  github-release-version-checker --repo hashicorp/terraform -c 1.11.1

  # Arkade (version-based policy)
  github-release-version-checker --repo alexellis/arkade -c 0.11.50

  # JSON output for automation
  github-release-version-checker -c 2.328.0 --json

  # CI mode for GitHub Actions
  github-release-version-checker --repo kubernetes/kubernetes -c 1.28.0 --ci

  # Audit local repositories
  github-release-version-checker audit-workflows local --path ~/src --repo-filter 'backend-api-*'

  # Ignore versions released in the last 7 days by default; set --cooldown 0 for immediate latest
  github-release-version-checker audit-workflows local --path ~/src --cooldown 0

  # Show the latest upstream SHA and copy-pasteable pinned refs
  github-release-version-checker audit-workflows local --path ~/src --pin-sha --view occurrences

  # Show every usage with absolute local workflow paths
  github-release-version-checker audit-workflows local --path ~/src --view occurrences

  # Audit container image references
  github-release-version-checker audit-containers local --path ~/src --pinning floating

  # Audit a GitHub owner boundary (user or organisation)
  github-release-version-checker audit-workflows owner my-org --only-floating --output json

  # Generate shell completions
  github-release-version-checker completion zsh
  github-release-version-checker completion bash`
)

// SetVersionInfo sets the version information from the main package
func SetVersionInfo(version, build, commit string) {
	appVersion = version
	buildTime = build
	gitCommit = commit
}

var rootCmd = newRootCommand()

func Execute() error {
	return rootCmd.Execute()
}

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "github-release-version-checker",
		Short:   rootShort,
		Long:    rootLong,
		Example: rootExample,
		RunE:    runCheck,
	}

	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	bindCheckFlags(cmd)
	cmd.AddCommand(
		buildCheckCommand(),
		buildAuditWorkflowsCommand(),
		buildAuditContainersCommand(),
	)

	return cmd
}

// detectGitHubToken attempts to find a GitHub token from multiple sources
func detectGitHubToken(providedToken string) string {
	return detectGitHubTokenForHost(providedToken, "github.com")
}

func detectGitHubTokenForHost(providedToken, host string) string {
	// 1. Use explicitly provided token (via -t flag or GITHUB_TOKEN env var)
	//    Note: GITHUB_TOKEN is automatically available in GitHub Actions
	if providedToken != "" {
		return providedToken
	}

	// 2. Try to get token from GitHub CLI
	ghToken, err := getGitHubCLIToken(host)
	if err == nil && ghToken != "" {
		return ghToken
	}

	// 3. No token found - will use unauthenticated requests
	return ""
}

// getGitHubCLIToken attempts to retrieve a token from the GitHub CLI
func getGitHubCLIToken(host string) (string, error) {
	args := []string{"auth", "token"}
	if host != "" && host != "github.com" {
		args = append(args, "--hostname", host)
	}

	cmd := exec.Command("gh", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	token := strings.TrimSpace(string(output))
	if token == "" {
		return "", fmt.Errorf("gh auth token returned empty")
	}

	return token, nil
}

func runCheck(cmd *cobra.Command, args []string) error {
	// Disable automatic usage printing on error
	cmd.SilenceUsage = true

	// Show version if requested
	if showVersion {
		fmt.Printf("github-release-version-checker %s\n", appVersion)
		fmt.Printf("Build time: %s\n", buildTime)
		fmt.Printf("Git commit: %s\n", gitCommit)
		return nil
	}

	// Validate inputs
	if criticalAgeDays >= maxAgeDays {
		return fmt.Errorf("critical-days (%d) must be less than max-days (%d)", criticalAgeDays, maxAgeDays)
	}

	// Auto-detect GitHub token from multiple sources if not provided
	token := detectGitHubToken(githubToken)

	// Resolve repository configuration
	var repoConfig *config.RepositoryConfig
	var err error

	if repository != "" {
		// Try predefined config first (for short names like "node", "k8s")
		repoConfig, err = config.GetPredefinedConfig(repository)
		if err != nil {
			// Not a predefined config, try parsing as owner/repo or URL
			repoConfig, err = config.ParseRepositoryString(repository)
			if err != nil {
				return fmt.Errorf("invalid repository: %w", err)
			}
		}
	} else {
		// Default to actions/runner
		repoConfig = &config.ConfigActionsRunner
	}

	// Override policy type if specified
	if policyType != "" {
		switch strings.ToLower(policyType) {
		case "days":
			repoConfig.PolicyType = config.PolicyTypeDays
		case "versions":
			repoConfig.PolicyType = config.PolicyTypeVersions
		default:
			return fmt.Errorf("invalid policy type %q: must be 'days' or 'versions'", policyType)
		}
	}

	// Override max versions if specified and using version policy
	if cmd.Flags().Changed("max-versions") {
		repoConfig.MaxVersionsBehind = maxVersions
	}

	// Override critical/max days if specified and using days policy
	if repoConfig.PolicyType == config.PolicyTypeDays {
		if cmd.Flags().Changed("critical-days") {
			repoConfig.CriticalDays = criticalAgeDays
		}
		if cmd.Flags().Changed("max-days") {
			repoConfig.MaxDays = maxAgeDays
		}
	}

	// Create GitHub client
	ghClient := client.NewClient(token, repoConfig.Owner, repoConfig.Repo)

	// Create cache manager (not used yet, but will be in future phases)
	_ = cache.NewManager(cachePath)

	// Create policy
	pol := policy.NewPolicy(repoConfig)

	// Create checker with policy
	versionChecker := checker.NewCheckerWithPolicy(ghClient, checker.Config{
		CriticalAgeDays: repoConfig.CriticalDays,
		MaxAgeDays:      repoConfig.MaxDays,
		NoCache:         noCache,
	}, pol)

	// Run analysis
	analysis, err := versionChecker.Analyse(cmd.Context(), comparisonVersion)
	if err != nil {
		// For JSON output, return error as JSON
		if jsonOutput {
			outputErrorJSON(err)
			return &ExitError{Code: 1, Silent: true}
		}

		// For CI output, return error immediately without formatting
		if ciOutput {
			return fmt.Errorf("%v", err)
		}

		// If invalid semantic version format, show helpful context
		if strings.Contains(err.Error(), "invalid comparison version") {
			red.Printf("\n❌ Error: %v\n\n", err)

			// Fetch latest release to show helpful info
			latestRelease, fetchErr := ghClient.GetLatestRelease(cmd.Context())
			if fetchErr == nil {
				yellow.Println("ℹ️  Semantic Version format: MAJOR.MINOR.PATCH")
				yellow.Printf("   Example: 2.326.0\n\n")
				yellow.Printf("💡 Most recent version is: v%s (Released %s)\n", latestRelease.Version, formatUKDate(latestRelease.PublishedAt))
			}

			return &ExitError{Code: 1, Silent: true}
		}

		// If version doesn't exist, show helpful context instead of just erroring
		if strings.Contains(err.Error(), "does not exist in GitHub releases") {
			red.Printf("\n❌ Error: %v\n\n", err)

			// Fetch latest release to show helpful info
			latestRelease, fetchErr := ghClient.GetLatestRelease(cmd.Context())
			if fetchErr == nil {
				yellow.Printf("💡 Use v%s (Released %s)\n", latestRelease.Version, formatUKDate(latestRelease.PublishedAt))

				// Show recent releases table if we can fetch them
				allReleases, fetchErr := ghClient.GetAllReleases(cmd.Context())
				if fetchErr == nil && len(allReleases) > 0 {
					// Create a minimal analysis just for displaying the table
					tempAnalysis := &checker.Analysis{
						LatestVersion: latestRelease.Version,
					}
					// Calculate recent releases for display
					tempChecker := checker.NewChecker(ghClient, checker.Config{})
					tempAnalysis.RecentReleases = tempChecker.CalculateRecentReleases(allReleases, latestRelease.Version, latestRelease.Version)

					printExpiryTable(tempAnalysis, comparisonVersion)
				}
			}

			return &ExitError{Code: 1, Silent: true}
		}
		// Check if it's an API error (rate limiting, network, etc.)
		if strings.Contains(err.Error(), "failed to fetch") || strings.Contains(err.Error(), "failed to get") || strings.Contains(err.Error(), "failed to list") {
			red.Printf("\n❌ Error: Unable to fetch release information from GitHub API\n\n")

			// Check if it's specifically a rate limit error
			if strings.Contains(err.Error(), "rate limit") {
				yellow.Println("⚠️  GitHub API Rate Limit Exceeded")
				yellow.Println()
				yellow.Println("   Unauthenticated requests are limited to 60 per hour.")
				yellow.Println("   Authenticated requests get 5,000 per hour.")
				yellow.Println()
				yellow.Println("💡 Authentication options (auto-detected in order):")
				yellow.Println("   1. Use the -t flag: github-release-version-checker -t YOUR_TOKEN")
				yellow.Println("   2. Set GITHUB_TOKEN environment variable")
				yellow.Println("   3. GitHub CLI: gh auth login (automatically detected)")
				yellow.Println("   4. GitHub Actions: GITHUB_TOKEN is auto-available")
				yellow.Println()
				yellow.Println("   Create a token at: https://github.com/settings/tokens")
				yellow.Println("   (Only needs 'public_repo' read access)")

				// Extract rate limit reset time if available
				if strings.Contains(err.Error(), "rate reset in") {
					start := strings.Index(err.Error(), "rate reset in")
					if start != -1 {
						resetInfo := err.Error()[start:]
						end := strings.Index(resetInfo, "]")
						if end != -1 {
							resetTime := resetInfo[:end]
							yellow.Printf("\n   Rate limit resets in: %s\n", strings.TrimPrefix(resetTime, "rate reset in "))
						}
					}
				}
			} else {
				// Other API errors (network, etc.)
				yellow.Println("ℹ️  Possible causes:")
				yellow.Println("   • Network connectivity issues")
				yellow.Println("   • GitHub API temporarily unavailable")
				yellow.Println("   • Firewall blocking api.github.com")
				yellow.Println()
				yellow.Printf("   Error details: %v\n", err)
			}

			return &ExitError{Code: 1, Silent: true}
		}

		return fmt.Errorf("analysis failed: %w", err)
	}

	// Output results
	if jsonOutput {
		return outputJSON(analysis)
	}

	if ciOutput {
		return outputCI(analysis)
	}

	return outputTerminal(analysis)
}

func outputJSON(analysis *checker.Analysis) error {
	data, err := analysis.MarshalJSON()
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func outputErrorJSON(err error) {
	errorJSON := fmt.Sprintf(`{
  "error": %q,
  "success": false
}`, err.Error())
	fmt.Println(errorJSON)
}

func outputCI(analysis *checker.Analysis) error {
	// Always print latest version first (for script compatibility)
	fmt.Println(analysis.LatestVersion)

	// If no comparison, we're done
	if analysis.ComparisonVersion == nil {
		return nil
	}

	status := analysis.Status()
	icon := getStatusIcon(status)

	// Build status line (same as terminal output but without colours)
	var statusLine string
	if analysis.IsLatest {
		comparisonDate := ""
		if analysis.ComparisonReleasedAt != nil {
			comparisonDate = fmt.Sprintf(" (%s)", formatUKDate(*analysis.ComparisonReleasedAt))
		}
		statusLine = fmt.Sprintf("Version %s%s is the latest version",
			analysis.ComparisonVersion,
			comparisonDate)
	} else {
		comparisonDate := ""
		if analysis.ComparisonReleasedAt != nil {
			comparisonDate = fmt.Sprintf(" (%s)", formatUKDate(*analysis.ComparisonReleasedAt))
		}

		expiryInfo := ""
		if analysis.FirstNewerReleaseDate != nil {
			expiryDate := analysis.FirstNewerReleaseDate.AddDate(0, 0, 30)

			if analysis.IsExpired {
				expiryInfo = fmt.Sprintf(" EXPIRED %s", formatUKDate(expiryDate))
			} else if analysis.IsCritical {
				daysLeft := 30 - analysis.DaysSinceUpdate
				expiryInfo = fmt.Sprintf(" EXPIRES %s (%d days)", formatUKDate(expiryDate), daysLeft)
			} else {
				expiryInfo = fmt.Sprintf(" expires %s", formatUKDate(expiryDate))
			}
		}

		latestDate := ""
		for _, r := range analysis.RecentReleases {
			if r.IsLatest {
				latestDate = fmt.Sprintf(" (Released %s)", formatUKDate(r.ReleasedAt))
				break
			}
		}

		statusLine = fmt.Sprintf("Version %s%s%s: Update to v%s%s",
			analysis.ComparisonVersion,
			comparisonDate,
			expiryInfo,
			analysis.LatestVersion,
			latestDate)
	}

	// Print GitHub Actions workflow commands
	fmt.Println()
	fmt.Println("::group::📊 Runner Version Check")
	fmt.Printf("Latest version: v%s\n", analysis.LatestVersion)
	fmt.Printf("Your version: v%s\n", analysis.ComparisonVersion)
	fmt.Printf("Status: %s\n", getStatusText(status))
	fmt.Println("::endgroup::")
	fmt.Println()

	// Use appropriate workflow command based on status
	switch status {
	case checker.StatusExpired:
		fmt.Printf("::error title=Runner Version Expired::%s %s\n", icon, statusLine)
	case checker.StatusCritical:
		fmt.Printf("::warning title=Runner Version Critical::%s %s\n", icon, statusLine)
	case checker.StatusWarning:
		fmt.Printf("::notice title=Runner Version Behind::%s %s\n", icon, statusLine)
	case checker.StatusCurrent:
		fmt.Printf("::notice title=Runner Version Current::%s %s\n", icon, statusLine)
	}

	// Print expiry table
	if len(analysis.RecentReleases) > 0 {
		fmt.Println()
		fmt.Println("::group::📅 Release Expiry Timeline")
		fmt.Printf("%-10s %-14s %-14s %s\n", "Version", "Release Date", "Expiry Date", "Status")

		for _, release := range analysis.RecentReleases {
			versionStr := release.Version.String()
			releasedStr := formatUKDate(release.ReleasedAt)

			var expiresStr string
			var statusStr string

			if release.IsLatest {
				expiresStr = "-"
				daysAgo := int(time.Since(release.ReleasedAt).Hours() / 24)
				statusStr = fmt.Sprintf("Latest (%s)", formatDaysAgo(daysAgo))
			} else if release.ExpiresAt != nil {
				expiresStr = formatUKDate(*release.ExpiresAt)

				if release.IsExpired {
					daysExpired := -release.DaysUntilExpiry
					statusStr = fmt.Sprintf("Expired %s", formatDaysAgo(daysExpired))
				} else {
					statusStr = fmt.Sprintf("Valid (%s left)", formatDaysInFuture(release.DaysUntilExpiry))
				}
			}

			arrow := ""
			if analysis.ComparisonVersion != nil && release.Version.Equal(analysis.ComparisonVersion) {
				arrow = "  [Your version]"
			}

			fmt.Printf("  %-10s %-14s %-14s %s%s\n", versionStr, releasedStr, expiresStr, statusStr, arrow)
		}

		// Add timestamp
		now := time.Now().UTC()
		timestamp := now.Format("2 Jan 2006 15:04:05 MST")
		fmt.Printf("\n  Checked at: %s\n", timestamp)

		fmt.Println("::endgroup::")
	}

	// Write markdown summary to $GITHUB_STEP_SUMMARY
	if summaryFile := os.Getenv("GITHUB_STEP_SUMMARY"); summaryFile != "" {
		if err := writeGitHubSummary(summaryFile, analysis); err != nil {
			fmt.Printf("::warning::Failed to write job summary: %v\n", err)
		}
	}

	return nil
}

func writeGitHubSummary(summaryFile string, analysis *checker.Analysis) error {
	f, err := os.OpenFile(summaryFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	status := analysis.Status()
	statusEmoji := getStatusIcon(status)
	statusText := getStatusText(status)

	// Write markdown summary
	fmt.Fprintf(f, "## %s Runner Version Status: %s\n\n", statusEmoji, statusText)

	// Summary table
	fmt.Fprintf(f, "| Metric | Value |\n")
	fmt.Fprintf(f, "|--------|-------|\n")
	fmt.Fprintf(f, "| Current Version | v%s |\n", analysis.ComparisonVersion)
	fmt.Fprintf(f, "| Latest Version | v%s |\n", analysis.LatestVersion)
	fmt.Fprintf(f, "| Status | %s %s |\n", statusEmoji, statusText)
	fmt.Fprintf(f, "| Releases Behind | %d |\n", analysis.ReleasesBehind)

	if analysis.DaysSinceUpdate > 0 {
		if analysis.IsExpired {
			daysOver := analysis.DaysSinceUpdate - analysis.MaxAgeDays
			fmt.Fprintf(f, "| Days Overdue | %d |\n", daysOver)
		} else {
			daysLeft := analysis.MaxAgeDays - analysis.DaysSinceUpdate
			fmt.Fprintf(f, "| Days Until Expiry | %d |\n", daysLeft)
		}
	}

	// Action required section
	switch status {
	case checker.StatusExpired:
		fmt.Fprintf(f, "\n### ⚠️ Action Required\n\n")
		fmt.Fprintf(f, "**Update to v%s or later immediately.** ", analysis.FirstNewerVersion)
		fmt.Fprintf(f, "GitHub will not queue jobs to runners with expired versions.\n")
	case checker.StatusCritical:
		daysLeft := analysis.MaxAgeDays - analysis.DaysSinceUpdate
		fmt.Fprintf(f, "\n### ⚠️ Update Soon\n\n")
		fmt.Fprintf(f, "Version expires in **%d days**. Update to v%s or later.\n", daysLeft, analysis.FirstNewerVersion)
	case checker.StatusWarning:
		fmt.Fprintf(f, "\n### ℹ️ Update Available\n\n")
		fmt.Fprintf(f, "A newer version (v%s) is available.\n", analysis.LatestVersion)
	}

	// Available updates
	if len(analysis.NewerReleases) > 0 {
		fmt.Fprintf(f, "\n### 📦 Available Updates\n\n")
		for _, release := range analysis.NewerReleases {
			releasedDaysAgo := int(time.Since(release.PublishedAt).Hours() / 24)
			fmt.Fprintf(f, "- [v%s](%s) - Released %s (%d days ago)\n",
				release.Version,
				release.URL,
				formatUKDate(release.PublishedAt),
				releasedDaysAgo)
		}
	}

	// Add timestamp
	now := time.Now().UTC()
	timestamp := now.Format("2 Jan 2006 15:04:05 MST")
	fmt.Fprintf(f, "\n*Checked at: %s*\n", timestamp)

	fmt.Fprintf(f, "\n---\n\n")

	return nil
}

func getStatusText(status checker.Status) string {
	switch status {
	case checker.StatusCurrent:
		return "Current"
	case checker.StatusWarning:
		return "Behind"
	case checker.StatusCritical:
		return "Critical"
	case checker.StatusExpired:
		return "Expired"
	default:
		return "Unknown"
	}
}

func outputTerminal(analysis *checker.Analysis) error {
	// Always print latest version first (for script compatibility)
	fmt.Println(analysis.LatestVersion)

	// If no comparison version provided
	if analysis.ComparisonVersion == nil {
		// In verbose mode, show recent releases table
		if verbose && len(analysis.RecentReleases) > 0 {
			fmt.Println()
			printExpiryTable(analysis, "")
		}
		return nil
	}

	// Print status
	fmt.Println()
	printStatus(analysis)

	// Print expiry table unless quiet mode
	if !quiet {
		printExpiryTable(analysis, "")
	}

	// Print verbose details if requested
	if verbose {
		fmt.Println()
		printDetails(analysis)
	}

	return nil
}

func printStatus(analysis *checker.Analysis) {
	status := analysis.Status()
	icon := getStatusIcon(status)
	colourFunc := getStatusColour(status)

	var statusLine string

	if analysis.ComparisonVersion == nil {
		// No comparison - just show latest
		statusLine = fmt.Sprintf("%s Latest version: v%s", icon, analysis.LatestVersion)
	} else if analysis.IsLatest {
		// On latest version
		if analysis.ComparisonReleasedAt != nil {
			statusLine = fmt.Sprintf("%s Version %s (%s) is the latest version",
				icon,
				analysis.ComparisonVersion,
				formatUKDate(*analysis.ComparisonReleasedAt))
		} else {
			statusLine = fmt.Sprintf("%s Version %s is the latest version",
				icon,
				analysis.ComparisonVersion)
		}
	} else {
		// Behind - construct full status line
		comparisonDate := ""
		if analysis.ComparisonReleasedAt != nil {
			comparisonDate = fmt.Sprintf(" (%s)", formatUKDate(*analysis.ComparisonReleasedAt))
		}

		// Check if using version-based policy
		isVersionPolicy := analysis.PolicyType == "versions"

		expiryInfo := ""
		if isVersionPolicy {
			// For version-based policies, show version skew info
			if analysis.IsExpired {
				expiryInfo = fmt.Sprintf(" UNSUPPORTED (%d minor versions behind)", analysis.MinorVersionsBehind)
			} else if analysis.IsCritical {
				expiryInfo = fmt.Sprintf(" CRITICAL (%d minor versions behind)", analysis.MinorVersionsBehind)
			} else if analysis.MinorVersionsBehind > 0 {
				expiryInfo = fmt.Sprintf(" (%d minor versions behind)", analysis.MinorVersionsBehind)
			}
		} else {
			// For days-based policies, show expiry dates
			if analysis.FirstNewerReleaseDate != nil {
				expiryDate := analysis.FirstNewerReleaseDate.AddDate(0, 0, 30)

				if analysis.IsExpired {
					expiryInfo = fmt.Sprintf(" EXPIRED %s", formatUKDate(expiryDate))
				} else if analysis.IsCritical {
					daysLeft := 30 - analysis.DaysSinceUpdate
					expiryInfo = fmt.Sprintf(" EXPIRES %s (%d days)", formatUKDate(expiryDate), daysLeft)
				} else {
					expiryInfo = fmt.Sprintf(" expires %s", formatUKDate(expiryDate))
				}
			}
		}

		latestDate := ""
		for _, r := range analysis.RecentReleases {
			if r.IsLatest {
				latestDate = fmt.Sprintf(" (Released %s)", formatUKDate(r.ReleasedAt))
				break
			}
		}

		statusLine = fmt.Sprintf("%s Version %s%s%s: Update to v%s%s",
			icon,
			analysis.ComparisonVersion,
			comparisonDate,
			expiryInfo,
			analysis.LatestVersion,
			latestDate)
	}

	colourFunc.Println(statusLine)
}

func printExpiryTable(analysis *checker.Analysis, phantomVersionStr string) {
	if len(analysis.RecentReleases) == 0 {
		return
	}

	isVersionPolicy := analysis.PolicyType == "versions"

	fmt.Println()
	if isVersionPolicy {
		cyan.Println("📋 Release Timeline")
		cyan.Println("─────────────────────────────────────────────────────")
		fmt.Printf("%-12s %-14s %s\n", "Version", "Release Date", "Status")
	} else {
		cyan.Println("📅 Release Expiry Timeline")
		cyan.Println("─────────────────────────────────────────────────────")
		fmt.Printf("%-10s %-14s %-14s %s\n", "Version", "Release Date", "Expiry Date", "Status")
	}

	// Parse phantom version if provided
	var phantomVersion *semver.Version
	if phantomVersionStr != "" {
		if v, err := semver.NewVersion(phantomVersionStr); err == nil {
			phantomVersion = v
		}
	}

	phantomPrinted := false
	for i, release := range analysis.RecentReleases {
		// Check if we should print phantom version before this release
		if phantomVersion != nil && !phantomPrinted && phantomVersion.LessThan(release.Version) {
			// Print phantom version row
			bold := colour.New(colour.Bold)
			if isVersionPolicy {
				bold.Printf("%-12s %-14s %s\n", phantomVersion.String(), "-", "❌ Does Not Exist  ← Your requested version")
			} else {
				bold.Printf("%-10s %-14s %-14s %s\n", phantomVersion.String(), "-", "-", "❌ Does Not Exist  ← Your requested version")
			}
			phantomPrinted = true
		}

		versionStr := release.Version.String()
		releasedStr := formatUKDate(release.ReleasedAt)

		var expiresStr string
		var statusStr string

		if isVersionPolicy {
			// For version-based policies, show version skew info
			daysAgo := int(time.Since(release.ReleasedAt).Hours() / 24)
			expiresStr = formatDaysAgo(daysAgo)

			if release.IsLatest {
				// Check if latest is also a .0 release
				if release.Version.Patch() == 0 {
					statusStr = "✅ Latest  ← Minor release"
				} else {
					statusStr = "✅ Latest"
				}
			} else {
				// Check if this is a .0 release (first release of a minor version)
				isMinorRelease := release.Version.Patch() == 0

				// Calculate minor version difference from LATEST version
				minorDiff := int(analysis.LatestVersion.Minor()) - int(release.Version.Minor())
				if release.Version.Major() == analysis.LatestVersion.Major() {
					if minorDiff > 0 {
						if isMinorRelease {
							statusStr = fmt.Sprintf("-%d minor  ← Minor release", minorDiff)
						} else {
							statusStr = fmt.Sprintf("-%d minor", minorDiff)
						}
					} else if minorDiff == 0 {
						// Same minor, check patch difference
						patchDiff := int(analysis.LatestVersion.Patch()) - int(release.Version.Patch())
						if patchDiff > 0 {
							if isMinorRelease {
								statusStr = fmt.Sprintf("-%d patch  ← Minor release", patchDiff)
							} else {
								statusStr = fmt.Sprintf("-%d patch", patchDiff)
							}
						} else {
							statusStr = "Same as latest"
						}
					} else {
						// Release is newer than latest? Shouldn't happen but handle it
						statusStr = ""
					}
				} else {
					// Different major version
					statusStr = fmt.Sprintf("v%d", release.Version.Major())
				}
			}
		} else {
			// For days-based policies, show expiry info
			if release.IsLatest {
				expiresStr = "-"
				daysAgo := int(time.Since(release.ReleasedAt).Hours() / 24)
				statusStr = fmt.Sprintf("✅ Latest (%s)", formatDaysAgo(daysAgo))
			} else if release.ExpiresAt != nil {
				expiresStr = formatUKDate(*release.ExpiresAt)

				if release.IsExpired {
					daysExpired := -release.DaysUntilExpiry
					statusStr = fmt.Sprintf("❌ Expired %s", formatDaysAgo(daysExpired))
				} else {
					statusStr = fmt.Sprintf("✅ Valid (%s left)", formatDaysInFuture(release.DaysUntilExpiry))
				}
			}
		}

		// Mark user's version with bold and arrow
		arrow := ""
		isUserVersion := analysis.ComparisonVersion != nil && release.Version.Equal(analysis.ComparisonVersion)
		if isUserVersion {
			arrow = "  ← Your version"
			// Format the whole line in bold
			bold := colour.New(colour.Bold)
			if isVersionPolicy {
				bold.Printf("%-12s %-14s %-16s %s%s\n", versionStr, releasedStr, expiresStr, statusStr, arrow)
			} else {
				bold.Printf("%-10s %-14s %-14s %s%s\n", versionStr, releasedStr, expiresStr, statusStr, arrow)
			}
		} else {
			if isVersionPolicy {
				fmt.Printf("%-12s %-14s %s%s\n", versionStr, releasedStr, statusStr, arrow)
			} else {
				fmt.Printf("%-10s %-14s %-14s %s%s\n", versionStr, releasedStr, expiresStr, statusStr, arrow)
			}
		}

		// Check if phantom should be printed after this (if it's the last release and phantom is greater)
		if phantomVersion != nil && !phantomPrinted && i == len(analysis.RecentReleases)-1 {
			// Print phantom version row
			bold := colour.New(colour.Bold)
			if isVersionPolicy {
				bold.Printf("%-12s %-14s %-16s %s\n", phantomVersion.String(), "-", "-", "❌ Does Not Exist  ← Your requested version")
			} else {
				bold.Printf("%-10s %-14s %-14s %s\n", phantomVersion.String(), "-", "-", "❌ Does Not Exist  ← Your requested version")
			}
			phantomPrinted = true
		}
	}

	// Add timestamp footer
	now := time.Now().UTC()
	timestamp := now.Format("2 Jan 2006 15:04:05 MST")
	grey.Printf("\nChecked at: %s\n", timestamp)
}

func printDetails(analysis *checker.Analysis) {
	cyan.Println("📊 Detailed Analysis")
	cyan.Println("─────────────────────────────────────")

	fmt.Printf("  Current version:      v%s\n", analysis.ComparisonVersion)
	fmt.Printf("  Latest version:       v%s\n", analysis.LatestVersion)
	fmt.Printf("  Status:               %s\n", analysis.Status())
	fmt.Printf("  Releases behind:      %d\n", analysis.ReleasesBehind)

	if analysis.FirstNewerVersion != nil {
		fmt.Printf("  First newer release:  v%s\n", analysis.FirstNewerVersion)
		if analysis.FirstNewerReleaseDate != nil {
			fmt.Printf("  Released on:          %s\n", analysis.FirstNewerReleaseDate.Format("2006-01-02"))
			fmt.Printf("  Days since update:    %d\n", analysis.DaysSinceUpdate)

			if analysis.DaysSinceUpdate < maxAgeDays {
				daysLeft := maxAgeDays - analysis.DaysSinceUpdate
				fmt.Printf("  Days until expired:   %d\n", daysLeft)
			} else {
				daysOver := analysis.DaysSinceUpdate - maxAgeDays
				fmt.Printf("  Days overdue:         %d\n", daysOver)
			}
		}
	}

	// Show available updates
	if len(analysis.NewerReleases) > 0 {
		fmt.Println()
		cyan.Println("📋 Available Updates")
		cyan.Println("─────────────────────────────────────")
		for _, release := range analysis.NewerReleases {
			releaseDate := release.PublishedAt.Format("2006-01-02")
			daysAgo := int(time.Since(release.PublishedAt).Hours() / 24)
			fmt.Printf("  • v%s (%s, %d days ago)\n", release.Version, releaseDate, daysAgo)
		}
	}
}

func getStatusIcon(status checker.Status) string {
	switch status {
	case checker.StatusCurrent:
		return "✅"
	case checker.StatusWarning:
		return "⚠️ "
	case checker.StatusCritical:
		return "🔶"
	case checker.StatusExpired:
		return "🚨"
	default:
		return "ℹ️ "
	}
}

func getStatusColour(status checker.Status) *colour.Color {
	switch status {
	case checker.StatusCurrent:
		return green
	case checker.StatusWarning:
		return yellow
	case checker.StatusCritical:
		return yellow
	case checker.StatusExpired:
		return red
	default:
		return cyan
	}
}
