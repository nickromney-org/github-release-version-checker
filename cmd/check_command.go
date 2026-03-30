package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

func bindCheckFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&comparisonVersion, "compare", "c", "", "version to compare against (e.g., 2.327.1)")
	cmd.Flags().IntVarP(&criticalAgeDays, "critical-days", "d", 12, "days before critical warning")
	cmd.Flags().IntVarP(&maxAgeDays, "max-days", "m", 30, "days before version expires")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	cmd.Flags().BoolVar(&ciOutput, "ci", false, "format output for CI/GitHub Actions")
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "quiet output (suppress expiry table)")
	cmd.Flags().StringVarP(&githubToken, "token", "t", os.Getenv("GITHUB_TOKEN"), "GitHub token (or GITHUB_TOKEN env var)")
	cmd.Flags().BoolVar(&showVersion, "version", false, "show version information")
	cmd.Flags().BoolVarP(&noCache, "no-cache", "n", false, "bypass embedded cache and always fetch from GitHub API")
	cmd.Flags().StringVarP(&repository, "repo", "r", "", "repository to check (format: owner/repo, e.g., 'kubernetes/kubernetes', 'pulumi/pulumi')")
	cmd.Flags().StringVar(&cachePath, "cache", "", "path to custom cache file")
	cmd.Flags().StringVar(&policyType, "policy", "", "policy type: 'days' or 'versions' (auto-detected if not specified)")
	cmd.Flags().IntVar(&maxVersions, "max-versions", 3, "maximum minor versions behind before expiry (for version-based policy)")
}

func buildCheckCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: rootShort,
		Long:  rootLong,
		RunE:  runCheck,
	}
	bindCheckFlags(cmd)
	return cmd
}
