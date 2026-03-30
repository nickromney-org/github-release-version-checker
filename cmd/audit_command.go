package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/nickromney-org/github-release-version-checker/internal/audit"
	"github.com/nickromney-org/github-release-version-checker/pkg/client"
	"github.com/spf13/cobra"
)

type auditCommandOptions struct {
	path              string
	repo              string
	owner             string
	org               string
	host              string
	token             string
	match             string
	visibility        string
	repoFilters       []string
	languages         []string
	kind              string
	pinning           []string
	format            string
	view              string
	failOn            string
	cooldownDays      int
	maxDepth          int
	onlyFloating      bool
	includeContainers bool
	resolveLatest     bool
	pinSHA            bool
	includeArchived   bool
	includeForks      bool
}

type auditCommandConfig struct {
	commandUse        string
	commandAliases    []string
	commandShort      string
	targetLabel       string
	defaultKind       string
	includeContainers bool
	includeKindFlag   bool
	includePinSHAFlag bool
	kindHelp          string
}

func buildAuditWorkflowsCommand() *cobra.Command {
	return buildAuditCommand(auditCommandConfig{
		commandUse:        "audit-workflows",
		commandAliases:    []string{"audit-actions"},
		commandShort:      "Audit GitHub Actions workflow dependencies",
		targetLabel:       "workflow dependencies",
		defaultKind:       string(audit.KindAll),
		includeContainers: false,
		includeKindFlag:   true,
		includePinSHAFlag: true,
		kindHelp:          "Dependency kind filter: action, reusable-workflow, docker-action, all",
	})
}

func buildAuditContainersCommand() *cobra.Command {
	return buildAuditCommand(auditCommandConfig{
		commandUse:        "audit-containers",
		commandShort:      "Audit container image references in GitHub Actions workflows",
		targetLabel:       "container references",
		defaultKind:       string(audit.KindContainer),
		includeContainers: true,
		includeKindFlag:   false,
		includePinSHAFlag: false,
	})
}

func buildAuditCommand(config auditCommandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:     config.commandUse,
		Aliases: config.commandAliases,
		Short:   config.commandShort,
		Long: config.commandShort + `

Use the local mode for checked-out repositories on disk. Use owner for remote bulk
audits across either a GitHub user or organisation account. The org command remains
available for organisation-specific usage.

Key flags:
- use --view summary for dense inventory and --view occurrences for file-by-file detail
- use -o json for machine-readable output and -o markdown for shareable clickable links
- use --match and --repo-filter to narrow large scans
- use --only-floating or --pinning to focus on specific pin styles
- use --cooldown to control how recent an upstream release may be before it is reported`,
		Example: auditCommandExample(config),
	}

	cmd.AddCommand(
		buildAuditLocalCommand(config),
		buildAuditRepoCommand(config),
		buildAuditOwnerCommand(config),
		buildAuditOrgCommand(config),
	)

	return cmd
}

func auditCommandExample(config auditCommandConfig) string {
	lines := []string{
		fmt.Sprintf("  github-release-version-checker %s local --path ~/src --only-floating", config.commandUse),
		fmt.Sprintf("  github-release-version-checker %s local --path ~/src --view occurrences -o markdown", config.commandUse),
		fmt.Sprintf("  github-release-version-checker %s owner my-org --repo-filter 'backend-api-*' -o json", config.commandUse),
	}
	if config.includePinSHAFlag {
		lines = append(lines, fmt.Sprintf("  github-release-version-checker %s local --path ~/src --pin-sha --view occurrences", config.commandUse))
	}
	return strings.Join(lines, "\n")
}

func buildAuditLocalCommand(config auditCommandConfig) *cobra.Command {
	options := &auditCommandOptions{
		path:              ".",
		host:              "github.com",
		format:            string(audit.FormatTable),
		view:              string(audit.ViewSummary),
		kind:              config.defaultKind,
		failOn:            string(audit.FailOnNone),
		cooldownDays:      7,
		maxDepth:          3,
		includeContainers: config.includeContainers,
		resolveLatest:     true,
	}

	cmd := &cobra.Command{
		Use:   "local [path]",
		Short: "Audit checked-out repositories on disk",
		Long: `Audit checked-out repositories on disk.

Local scans do not need a token to discover workflow usage. A token only affects
optional upstream lookups such as the latest eligible release or tag.

In local occurrences output, workflow paths are rendered as absolute filesystem
paths so terminals and IDEs can open them directly.`,
		Example: auditLocalExample(config),
		Args:    cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return applyAuditLocalArgs(options, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			auditOptions, err := options.toAuditOptions(audit.ModeLocal)
			if err != nil {
				return &ExitError{Code: 2, Msg: err.Error()}
			}

			scanner, err := newAuditScanner(nil, options)
			if err != nil {
				return &ExitError{Code: 2, Msg: err.Error()}
			}
			result, err := scanner.ScanLocal(cmd.Context(), auditOptions)
			if err != nil {
				return &ExitError{Code: 2, Msg: err.Error()}
			}
			return renderAuditResult(cmd, result, auditOptions)
		},
	}

	bindSharedAuditFlags(cmd, options, config)
	bindRemoteAuditFlags(cmd, options)
	cmd.Flags().StringVar(&options.path, "path", ".", "Repository root or workspace directory to scan")
	cmd.Flags().IntVar(&options.maxDepth, "max-depth", 3, "Maximum directory depth when discovering local repositories")
	return cmd
}

func buildAuditRepoCommand(config auditCommandConfig) *cobra.Command {
	options := &auditCommandOptions{
		host:              "github.com",
		format:            string(audit.FormatTable),
		view:              string(audit.ViewSummary),
		kind:              config.defaultKind,
		failOn:            string(audit.FailOnNone),
		cooldownDays:      7,
		maxDepth:          3,
		includeContainers: config.includeContainers,
		resolveLatest:     true,
	}

	cmd := &cobra.Command{
		Use:     "repo [owner/repo]",
		Short:   "Audit " + config.targetLabel + " in a single remote repository",
		Example: auditRepoExample(config),
		Args:    cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return applyAuditRepoArgs(options, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			auditOptions, err := options.toAuditOptions(audit.ModeRepo)
			if err != nil {
				return &ExitError{Code: 2, Msg: err.Error()}
			}

			remoteSource, err := newRemoteAuditSource(options.token, options.host)
			if err != nil {
				return &ExitError{Code: 2, Msg: err.Error()}
			}

			scanner, err := newAuditScanner(remoteSource, options)
			if err != nil {
				return &ExitError{Code: 2, Msg: err.Error()}
			}
			result, err := scanner.ScanRepo(cmd.Context(), auditOptions)
			if err != nil {
				return &ExitError{Code: 2, Msg: err.Error()}
			}
			return renderAuditResult(cmd, result, auditOptions)
		},
	}

	bindSharedAuditFlags(cmd, options, config)
	bindRemoteAuditFlags(cmd, options)
	cmd.Flags().StringVar(&options.repo, "repo", "", "Repository to audit in owner/repo format")
	return cmd
}

func buildAuditOwnerCommand(config auditCommandConfig) *cobra.Command {
	options := &auditCommandOptions{
		host:              "github.com",
		format:            string(audit.FormatTable),
		view:              string(audit.ViewSummary),
		kind:              config.defaultKind,
		failOn:            string(audit.FailOnNone),
		cooldownDays:      7,
		maxDepth:          3,
		includeContainers: config.includeContainers,
		resolveLatest:     true,
	}

	cmd := &cobra.Command{
		Use:   "owner [owner]",
		Short: "Audit " + config.targetLabel + " across a GitHub owner account",
		Long: `Audit workflow dependencies across a GitHub owner account.

Owner means either a GitHub user or a GitHub organisation. For user owners,
public repositories are scanned by default. If the token or GitHub CLI auth
belongs to that same user, private owned repositories are included too.

Prefer owner for new remote bulk scans.`,
		Example: auditOwnerExample(config),
		Args:    cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return applyAuditOwnerArgs(options, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			auditOptions, err := options.toAuditOptions(audit.ModeOwner)
			if err != nil {
				return &ExitError{Code: 2, Msg: err.Error()}
			}

			remoteSource, err := newRemoteAuditSource(options.token, options.host)
			if err != nil {
				return &ExitError{Code: 2, Msg: err.Error()}
			}

			scanner, err := newAuditScanner(remoteSource, options)
			if err != nil {
				return &ExitError{Code: 2, Msg: err.Error()}
			}
			result, err := scanner.ScanOwner(cmd.Context(), auditOptions)
			if err != nil {
				return &ExitError{Code: 2, Msg: err.Error()}
			}
			return renderAuditResult(cmd, result, auditOptions)
		},
	}

	bindSharedAuditFlags(cmd, options, config)
	bindRemoteAuditFlags(cmd, options)
	cmd.Flags().StringVar(&options.owner, "owner", "", "Owner account to audit (user or organisation)")
	cmd.Flags().StringVar(&options.visibility, "visibility", "", "Repository visibility filter: public, private, internal, all (default auto)")
	cmd.Flags().BoolVar(&options.includeArchived, "include-archived", false, "Include archived repositories")
	cmd.Flags().BoolVar(&options.includeForks, "include-forks", false, "Include forked repositories")
	cmd.Flags().StringArrayVar(&options.languages, "language", nil, "Repository language filter (repeatable)")
	return cmd
}

func buildAuditOrgCommand(config auditCommandConfig) *cobra.Command {
	options := &auditCommandOptions{
		host:              "github.com",
		format:            string(audit.FormatTable),
		view:              string(audit.ViewSummary),
		kind:              config.defaultKind,
		failOn:            string(audit.FailOnNone),
		visibility:        "all",
		cooldownDays:      7,
		maxDepth:          3,
		includeContainers: config.includeContainers,
		resolveLatest:     true,
	}

	cmd := &cobra.Command{
		Use:   "org [organisation]",
		Short: "Audit " + config.targetLabel + " across a GitHub organisation",
		Long: `Audit workflow dependencies across a GitHub organisation.

This is the organisation-specific form. Prefer owner for new usage unless you
specifically want an organisation-only command.`,
		Example: auditOrgExample(config),
		Args:    cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return applyAuditOrgArgs(options, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			auditOptions, err := options.toAuditOptions(audit.ModeOrg)
			if err != nil {
				return &ExitError{Code: 2, Msg: err.Error()}
			}

			remoteSource, err := newRemoteAuditSource(options.token, options.host)
			if err != nil {
				return &ExitError{Code: 2, Msg: err.Error()}
			}

			scanner, err := newAuditScanner(remoteSource, options)
			if err != nil {
				return &ExitError{Code: 2, Msg: err.Error()}
			}
			result, err := scanner.ScanOrg(cmd.Context(), auditOptions)
			if err != nil {
				return &ExitError{Code: 2, Msg: err.Error()}
			}
			return renderAuditResult(cmd, result, auditOptions)
		},
	}

	bindSharedAuditFlags(cmd, options, config)
	bindRemoteAuditFlags(cmd, options)
	cmd.Flags().StringVar(&options.org, "org", "", "Organisation to audit")
	cmd.Flags().StringVar(&options.visibility, "visibility", "all", "Repository visibility filter: all, public, private, internal")
	cmd.Flags().BoolVar(&options.includeArchived, "include-archived", false, "Include archived repositories")
	cmd.Flags().BoolVar(&options.includeForks, "include-forks", false, "Include forked repositories")
	cmd.Flags().StringArrayVar(&options.languages, "language", nil, "Repository language filter (repeatable)")
	return cmd
}

func applyAuditLocalArgs(options *auditCommandOptions, args []string) error {
	if len(args) == 0 {
		return nil
	}
	if options.path != "." && options.path != "" && options.path != args[0] {
		return fmt.Errorf("path specified twice: positional %q and --path %q", args[0], options.path)
	}
	options.path = args[0]
	return nil
}

func auditLocalExample(config auditCommandConfig) string {
	lines := []string{
		fmt.Sprintf("  github-release-version-checker %s local --path .", config.commandUse),
		fmt.Sprintf("  github-release-version-checker %s local --path ~/src --view occurrences", config.commandUse),
		fmt.Sprintf("  github-release-version-checker %s local --path ~/src --view occurrences -o markdown", config.commandUse),
	}
	if config.includePinSHAFlag {
		lines = append(lines, fmt.Sprintf("  github-release-version-checker %s local --path ~/src --cooldown 0 --pin-sha", config.commandUse))
	} else {
		lines = append(lines, fmt.Sprintf("  github-release-version-checker %s local --path ~/src --cooldown 0", config.commandUse))
	}
	return strings.Join(lines, "\n")
}

func auditRepoExample(config auditCommandConfig) string {
	return strings.Join([]string{
		fmt.Sprintf("  github-release-version-checker %s repo owner/repo", config.commandUse),
		fmt.Sprintf("  github-release-version-checker %s repo owner/repo --host github.example.com", config.commandUse),
	}, "\n")
}

func auditOwnerExample(config auditCommandConfig) string {
	return strings.Join([]string{
		fmt.Sprintf("  github-release-version-checker %s owner nickromney", config.commandUse),
		fmt.Sprintf("  github-release-version-checker %s owner nickromney-org", config.commandUse),
		fmt.Sprintf("  github-release-version-checker %s owner nickromney --visibility private", config.commandUse),
		fmt.Sprintf("  github-release-version-checker %s owner nickromney-org --repo-filter 'backend-api-*' -o json", config.commandUse),
	}, "\n")
}

func auditOrgExample(config auditCommandConfig) string {
	return strings.Join([]string{
		fmt.Sprintf("  github-release-version-checker %s org my-org", config.commandUse),
		fmt.Sprintf("  github-release-version-checker %s org my-org --repo-filter 'backend-api-*' -o csv", config.commandUse),
	}, "\n")
}

func applyAuditRepoArgs(options *auditCommandOptions, args []string) error {
	return applyAuditSelectorArg("repository", args, &options.repo, "--repo")
}

func applyAuditOwnerArgs(options *auditCommandOptions, args []string) error {
	return applyAuditSelectorArg("owner", args, &options.owner, "--owner")
}

func applyAuditOrgArgs(options *auditCommandOptions, args []string) error {
	return applyAuditSelectorArg("organisation", args, &options.org, "--org")
}

func applyAuditSelectorArg(label string, args []string, value *string, flagName string) error {
	if len(args) > 1 {
		return fmt.Errorf("expected at most one %s argument", label)
	}
	if len(args) == 1 {
		if *value != "" && *value != args[0] {
			return fmt.Errorf("%s specified twice: positional %q and %s %q", label, args[0], flagName, *value)
		}
		*value = args[0]
	}
	if strings.TrimSpace(*value) == "" {
		return fmt.Errorf("%s is required: provide it as a positional argument or with %s", label, flagName)
	}
	return nil
}

func bindSharedAuditFlags(cmd *cobra.Command, options *auditCommandOptions, config auditCommandConfig) {
	cmd.Flags().StringVarP(&options.format, "output", "o", string(audit.FormatTable), "Output format: table, json, csv, markdown")
	cmd.Flags().StringVar(&options.view, "view", string(audit.ViewSummary), "Output view: summary, occurrences")
	cmd.Flags().StringVar(&options.match, "match", "", "Partial match filter for dependency name, ref, or workflow location")
	cmd.Flags().StringArrayVar(&options.repoFilters, "repo-filter", nil, "Repository name/full-name glob filter (repeatable)")
	if config.includeKindFlag {
		cmd.Flags().StringVar(&options.kind, "kind", config.defaultKind, config.kindHelp)
	}
	cmd.Flags().StringArrayVar(&options.pinning, "pinning", nil, "Reference pinning filter (repeatable): floating, semver, sha")
	cmd.Flags().BoolVar(&options.onlyFloating, "only-floating", false, "Only report floating references")
	cmd.Flags().BoolVar(&options.resolveLatest, "resolve-latest", true, "Resolve the highest eligible upstream version after applying cooldown")
	cmd.Flags().IntVar(&options.cooldownDays, "cooldown", 7, "Only consider upstream versions at least this many days old; use 0 to disable")
	if config.includePinSHAFlag {
		cmd.Flags().BoolVar(&options.pinSHA, "pin-sha", false, "Resolve the latest upstream commit SHA and show ready-to-paste pinned refs")
	}
	cmd.Flags().StringVar(&options.failOn, "fail-on", string(audit.FailOnNone), "Fail when findings match: none, floating")
}

func bindRemoteAuditFlags(cmd *cobra.Command, options *auditCommandOptions) {
	cmd.Flags().StringVarP(&options.token, "token", "t", os.Getenv("GITHUB_TOKEN"), "GitHub token (or GITHUB_TOKEN env var)")
	cmd.Flags().StringVar(&options.host, "host", "github.com", "GitHub host (github.com or GitHub Enterprise hostname)")
}

func newRemoteAuditSource(token, host string) (*audit.GitHubRemoteSource, error) {
	clientToken := detectGitHubTokenForHost(token, host)
	ghClient, err := client.NewClientWithOptions(client.ClientOptions{
		Token: clientToken,
		Host:  host,
	})
	if err != nil {
		return nil, err
	}
	return audit.NewGitHubRemoteSource(ghClient.GitHub()), nil
}

func newAuditScanner(remoteSource audit.RemoteSource, options *auditCommandOptions) (*audit.Scanner, error) {
	scanner := audit.NewScanner(remoteSource)
	if !options.resolveLatest && !options.pinSHA {
		return scanner, nil
	}

	clientToken := detectGitHubTokenForHost(options.token, options.host)
	ghClient, err := client.NewClientWithOptions(client.ClientOptions{
		Token: clientToken,
		Host:  options.host,
	})
	if err != nil {
		return nil, err
	}
	scanner.Latest = audit.NewGitHubLatestResolver(ghClient.GitHub())
	return scanner, nil
}

func renderAuditResult(cmd *cobra.Command, result *audit.Result, options audit.Options) error {
	var output bytes.Buffer
	if err := audit.Render(&output, result, options.Format, options.View); err != nil {
		return &ExitError{Code: 2, Msg: err.Error()}
	}
	printAuditWarnings(cmd.ErrOrStderr(), result, options)
	if _, err := io.Copy(cmd.OutOrStdout(), &output); err != nil {
		return &ExitError{Code: 2, Msg: err.Error()}
	}
	if options.FailOn == audit.FailOnFloating && result.HasFloating() {
		return &ExitError{Code: 3, Silent: true}
	}
	return nil
}

func printAuditWarnings(w io.Writer, result *audit.Result, options audit.Options) {
	if len(result.Warnings) == 0 {
		return
	}
	if options.Format == audit.FormatJSON {
		return
	}

	for _, warning := range result.Warnings {
		fmt.Fprintf(w, "warning: skipped %s: %s\n", warning.File, strings.TrimSpace(warning.Message))
	}
}

func (o *auditCommandOptions) toAuditOptions(mode audit.Mode) (audit.Options, error) {
	format, err := parseAuditFormat(o.format)
	if err != nil {
		return audit.Options{}, err
	}
	view, err := parseAuditView(o.view)
	if err != nil {
		return audit.Options{}, err
	}
	kind, err := parseAuditKind(o.kind)
	if err != nil {
		return audit.Options{}, err
	}
	pinning, err := parseAuditPinning(o.pinning)
	if err != nil {
		return audit.Options{}, err
	}
	failOn, err := parseAuditFailOn(o.failOn)
	if err != nil {
		return audit.Options{}, err
	}
	if o.onlyFloating && len(pinning) > 0 {
		return audit.Options{}, fmt.Errorf("--only-floating cannot be combined with --pinning; use --pinning floating")
	}
	if o.cooldownDays < 0 {
		return audit.Options{}, fmt.Errorf("--cooldown must be 0 or greater")
	}
	if !o.includeContainers && kind == audit.KindContainer {
		return audit.Options{}, fmt.Errorf("--kind container is not available for audit-workflows; use audit-containers instead")
	}

	return audit.Options{
		Mode:              mode,
		Path:              o.path,
		Repo:              o.repo,
		Owner:             o.owner,
		Org:               o.org,
		Host:              o.host,
		Match:             o.match,
		RepoFilters:       o.repoFilters,
		Kind:              kind,
		Pinning:           pinning,
		ResolveLatest:     o.resolveLatest || o.pinSHA,
		ResolveSHA:        o.pinSHA,
		CooldownDays:      o.cooldownDays,
		IncludeContainers: o.includeContainers,
		OnlyFloating:      o.onlyFloating,
		View:              view,
		Format:            format,
		FailOn:            failOn,
		MaxDepth:          o.maxDepth,
		Visibility:        o.visibility,
		IncludeArchived:   o.includeArchived,
		IncludeForks:      o.includeForks,
		Languages:         o.languages,
	}, nil
}

func parseAuditFormat(value string) (audit.Format, error) {
	switch value {
	case string(audit.FormatTable), "":
		return audit.FormatTable, nil
	case string(audit.FormatJSON):
		return audit.FormatJSON, nil
	case string(audit.FormatCSV):
		return audit.FormatCSV, nil
	case string(audit.FormatMarkdown):
		return audit.FormatMarkdown, nil
	default:
		return "", fmt.Errorf("invalid format %q: must be table, json, csv, or markdown", value)
	}
}

func parseAuditView(value string) (audit.View, error) {
	switch value {
	case string(audit.ViewSummary), "":
		return audit.ViewSummary, nil
	case string(audit.ViewOccurrences):
		return audit.ViewOccurrences, nil
	default:
		return "", fmt.Errorf("invalid view %q: must be summary or occurrences", value)
	}
}

func parseAuditKind(value string) (audit.Kind, error) {
	switch value {
	case string(audit.KindAll), "":
		return audit.KindAll, nil
	case string(audit.KindAction):
		return audit.KindAction, nil
	case string(audit.KindReusableWorkflow):
		return audit.KindReusableWorkflow, nil
	case string(audit.KindDockerAction):
		return audit.KindDockerAction, nil
	case string(audit.KindContainer):
		return audit.KindContainer, nil
	default:
		return "", fmt.Errorf("invalid kind %q: must be action, reusable-workflow, docker-action, container, or all", value)
	}
}

func parseAuditPinning(values []string) ([]audit.Pinning, error) {
	if len(values) == 0 {
		return nil, nil
	}

	parsed := make([]audit.Pinning, 0, len(values))
	for _, value := range values {
		switch value {
		case string(audit.PinningFloating), "latest", "main", "master":
			parsed = append(parsed, audit.PinningFloating)
		case string(audit.PinningSemver):
			parsed = append(parsed, audit.PinningSemver)
		case string(audit.PinningSHA), "sha-pinned":
			parsed = append(parsed, audit.PinningSHA)
		default:
			return nil, fmt.Errorf("invalid pinning %q: must be floating, semver, or sha", value)
		}
	}

	return parsed, nil
}

func parseAuditFailOn(value string) (audit.FailOn, error) {
	switch value {
	case string(audit.FailOnNone), "":
		return audit.FailOnNone, nil
	case string(audit.FailOnFloating):
		return audit.FailOnFloating, nil
	default:
		return "", fmt.Errorf("invalid fail-on %q: must be none or floating", value)
	}
}
