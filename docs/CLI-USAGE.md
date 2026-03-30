# CLI Usage Guide

Complete guide to using the GitHub Release Version Checker command-line interface.

## Table of Contents

- [Basic Usage](#basic-usage)
- [Supported Repositories](#supported-repositories)
- [Workflow Auditing](#workflow-auditing)
- [Output Formats](#output-formats)
- [Command Line Options](#command-line-options)
- [Examples](#examples)
- [Integration Patterns](#integration-patterns)

## Basic Usage

### Check Latest Version

Get the latest version without comparison:

```bash
$ github-release-version-checker
1.329.0
```

Perfect for scripts:

```bash
LATEST_VERSION=$(github-release-version-checker)
echo "Latest runner version is: $LATEST_VERSION"
```

### Check Specific Version

Compare a version against the latest:

```bash
github-release-version-checker -c 2.328.0
```

## Supported Repositories

### GitHub Actions Runner (Default)

Uses days-based policy (30-day update requirement):

```bash
# Check latest version
github-release-version-checker

# Check a specific version
github-release-version-checker -c 2.328.0
```

### Kubernetes

Uses version-based policy (3 minor versions behind):

```bash
github-release-version-checker --repo kubernetes/kubernetes -c 1.31.12
github-release-version-checker --repo k8s -c 1.28.0
```

### Node.js

Uses version-based policy (3 major versions behind):

```bash
github-release-version-checker --repo nodejs/node -c v20.0.0
github-release-version-checker --repo node -c v18.20.0
```

### Pulumi

```bash
github-release-version-checker --repo pulumi/pulumi -c 3.204.0
github-release-version-checker --repo pulumi -c 3.200.0
```

### HashiCorp Terraform

```bash
github-release-version-checker --repo hashicorp/terraform -c 1.11.1
```

### Arkade

```bash
github-release-version-checker --repo alexellis/arkade -c 0.11.50
```

### Any GitHub Repository

```bash
# Using owner/repo format
github-release-version-checker --repo owner/repo -c 1.0.0

# Using GitHub URL
github-release-version-checker --repo https://github.com/owner/repo -c 1.0.0
```

## Workflow Auditing

Audit GitHub Actions usage, reusable workflows, and workflow container images.

### Local-First Audits

Scan a checked-out repository or workspace with no GitHub token:

```bash
# Single repository
github-release-version-checker audit-workflows local --path .

# Workspace with repo-name filtering
github-release-version-checker audit-workflows local --path ~/src --repo-filter 'backend-api-*'

# Only floating refs such as @main, @master, @latest
github-release-version-checker audit-workflows local --path ~/src --only-floating

# Use the default 7-day cooldown for upstream version resolution
github-release-version-checker audit-workflows local --path ~/src

# Disable the cooldown and take the immediate latest upstream version
github-release-version-checker audit-workflows local --path ~/src --cooldown 0

# Resolve the latest upstream SHA and show pinned uses lines
github-release-version-checker audit-workflows local --path ~/src --pin-sha --view occurrences

# Show absolute workflow paths for direct terminal or IDE opening
github-release-version-checker audit-workflows local --path ~/src --view occurrences
```

### Remote Repository Audits

```bash
# Single repository on github.com
github-release-version-checker audit-workflows repo --repo owner/repo

# GitHub Enterprise host
github-release-version-checker audit-workflows repo --repo owner/repo --host github.example.com
```

### Owner Audits

```bash
# Entire owner boundary (user or organisation)
github-release-version-checker audit-workflows owner my-org

# Filter repositories and export CSV
github-release-version-checker audit-workflows owner my-org --repo-filter 'backend-api-*' --output csv

# User owners scan public repos by default
github-release-version-checker audit-workflows owner nickromney --repo-filter 'thesis-*'

# If the token belongs to that user owner, private owned repos are included too
github-release-version-checker audit-workflows owner nickromney --visibility private

# org remains available for organisation-specific usage
github-release-version-checker audit-workflows org my-org
```

Audit output views:

- `--view summary` aggregates each dependency/ref pair with repo and occurrence counts
- `--view occurrences` lists every repo, workflow path, job, step, and line number
- local `--view occurrences` renders workflow paths as absolute filesystem paths
- `LATEST AGE` shows how many days old the selected upstream release is
- `LATEST` means the highest eligible upstream release or tag after cooldown; it is not restricted to the current major line already in use
- `--pin-sha` resolves the latest upstream commit SHA and adds copy-pasteable pinned refs for actions
- `--cooldown` defaults to `7 days`; use `--cooldown 0` to disable it

Audit output formats:

- `--output table`
- `--output json`
- `--output csv`
- `--output markdown`

### Shell Completions

The CLI includes the built-in Cobra completion command:

```bash
# zsh
github-release-version-checker completion zsh > ~/.zsh/completions/_github-release-version-checker

# bash
github-release-version-checker completion bash > ~/.local/share/bash-completion/completions/github-release-version-checker

# fish
github-release-version-checker completion fish > ~/.config/fish/completions/github-release-version-checker.fish
```

Use `github-release-version-checker completion --help` for the full set of supported shells.

## Output Formats

### Terminal Output (Default)

Human-readable colourised output:

```bash
$ github-release-version-checker -c 2.327.1
1.329.0

 Version 2.327.1 (25 Jul 2025) EXPIRED 12 Sep 2025: Update to v2.329.0 (Released 14 Oct 2025)

 Release Expiry Timeline
─────────────────────────────────────────────────────
Version Release Date Expiry Date Status
1.327.0 22 Jul 2025 24 Aug 2025 Expired 67 days ago
1.327.1 25 Jul 2025 12 Sep 2025 Expired 48 days ago ← Your version
1.328.0 13 Aug 2025 13 Nov 2025 Valid (13 days left)
1.329.0 14 Oct 2025 - Latest

Checked at: 3 Nov 2025 17:44:55 UTC
```

### Quiet Mode

Suppress the timeline table:

```bash
$ github-release-version-checker -c 2.327.1 -q
1.329.0

 Version 2.327.1 (25 Jul 2025) EXPIRED 12 Sep 2025: Update to v2.329.0 (Released 14 Oct 2025)
```

### Verbose Output

Show detailed analysis:

```bash
$ github-release-version-checker -c 2.328.0 -v
1.329.0

 Version 2.328.0 Warning: 1 release behind

 Update available: v2.329.0
 Released: Oct 14, 2024 (3 days ago)
 Latest version: v2.329.0

 Detailed Analysis
─────────────────────────────────────
Current version: v2.328.0
Latest version: v2.329.0
Status: warning
Releases behind: 1
First newer release: v2.329.0
Released on: 2024-10-14
Days since update: 3
Days until expired: 27

 Available Updates
─────────────────────────────────────
 • v2.329.0 (2024-10-14, 3 days ago)
```

### JSON Output

Machine-readable output for automation:

```bash
$ github-release-version-checker -c 2.327.1 --json
{
 "latest_version": "2.329.0",
 "comparison_version": "2.327.1",
 "is_latest": false,
 "is_expired": true,
 "is_critical": false,
 "releases_behind": 2,
 "days_since_update": 65,
 "first_newer_version": "2.328.0",
 "first_newer_release_date": "2024-08-13T10:30:00Z",
 "status": "expired",
 "message": "Version 2.327.1 EXPIRED: 2 releases behind AND 35 days overdue",
 "critical_age_days": 12,
 "max_age_days": 30,
 "policy_type": "days"
}
```

### CI/GitHub Actions Output

Formatted for GitHub Actions with collapsible sections and annotations:

```bash
$ github-release-version-checker -c 2.327.1 --ci
1.329.0

::group:: Runner Version Check
Latest version: v2.329.0
Your version: v2.327.1
Status: Expired
::endgroup::

::error title=Runner Version Expired:: Version 2.327.1 EXPIRED! (2 releases behind AND 35 days overdue)
::error::Update required: v2.328.0 was released 65 days ago
::error::Latest version: v2.329.0

::group:: Available Updates
 • v2.329.0 (2024-10-14, 3 days ago) [Latest]
 • v2.328.0 (2024-08-13, 65 days ago) [First newer release]
::endgroup::
```

Plus a beautiful markdown summary in the GitHub Actions job summary!

## Command Line Options

```bash
Usage:
 github-release-version-checker [flags]

Flags:
 -c, --compare string version to compare against (e.g., 2.327.1)
 --repo string repository to check (default: actions/runner)
 Examples: k8s, node, owner/repo, github.com/owner/repo
 -d, --critical-days int days before critical warning (default 12)
 -m, --max-days int days before version expires (default 30)
 -v, --verbose verbose output with detailed analysis
 --json output as JSON for automation
 --ci format output for CI/GitHub Actions
 -q, --quiet quiet output (suppress timeline table)
 -n, --no-cache bypass embedded cache and always fetch from GitHub API
 -t, --token string GitHub token (or set GITHUB_TOKEN env var)
 --version show version information
 -h, --help help for github-release-version-checker
```

## Examples

### Example 1: Current Version

```bash
$ github-release-version-checker -c 2.329.0
1.329.0

 Version 2.329.0 (14 Oct 2025) is the latest version
```

### Example 2: Warning Status

```bash
$ github-release-version-checker -c 2.328.0
1.329.0

 Version 2.328.0 (13 Aug 2025) expires 12 Sep 2025: Update to v2.329.0

 Release Expiry Timeline
─────────────────────────────────────────────────────
Version Release Date Expiry Date Status
1.328.0 13 Aug 2025 12 Sep 2025 Valid (9 days left) ← Your version
1.329.0 14 Oct 2025 - Latest
```

### Example 3: Critical Status

```bash
$ github-release-version-checker -c 2.327.0
1.329.0

 Version 2.327.0 (22 Jul 2025) EXPIRES 24 Aug 2025 (-68 days): Update to v2.329.0

 Release Expiry Timeline
─────────────────────────────────────────────────────
Version Release Date Expiry Date Status
1.327.0 22 Jul 2025 24 Aug 2025 EXPIRES in -68 days ← Your version
1.327.1 25 Jul 2025 12 Sep 2025 Valid (-45 days)
1.328.0 13 Aug 2025 13 Nov 2025 Valid (13 days left)
1.329.0 14 Oct 2025 - Latest
```

### Example 4: Expired Status

```bash
$ github-release-version-checker -c 2.327.1
1.329.0

 Version 2.327.1 (25 Jul 2025) EXPIRED 12 Sep 2025: Update to v2.329.0 (Released 14 Oct 2025)

 Release Expiry Timeline
─────────────────────────────────────────────────────
Version Release Date Expiry Date Status
1.327.0 22 Jul 2025 24 Aug 2025 Expired 67 days ago
1.327.1 25 Jul 2025 12 Sep 2025 Expired 48 days ago ← Your version
1.328.0 13 Aug 2025 13 Nov 2025 Valid (13 days left)
1.329.0 14 Oct 2025 - Latest
```

### Example 5: Version-Based Policy (Kubernetes)

```bash
$ github-release-version-checker --repo k8s -c 1.31.13
1.34.1

 Version 1.31.13 (10 Sep 2025) CRITICAL (3 minor versions behind): Update to v1.34.1 (Released 10 Sep 2025)

 Release Timeline
─────────────────────────────────────────────────────
Version Release Date Status
1.31.0 13 Aug 2024 -3 minor ← Minor release
1.31.13 10 Sep 2025 54 days ago -3 minor ← Your version
1.32.0 11 Dec 2024 -2 minor ← Minor release
1.32.9 10 Sep 2025 -2 minor
1.33.0 23 Apr 2025 -1 minor ← Minor release
1.33.5 10 Sep 2025 -1 minor
1.34.0 27 Aug 2025 -1 patch ← Minor release
1.34.1 10 Sep 2025 Latest

Checked at: 3 Nov 2025 17:44:55 UTC
```

### Example 6: Using GitHub Token

Avoid rate limiting (60 req/hour → 5000 req/hour):

```bash
# Via flag
github-release-version-checker -t $GITHUB_TOKEN -c 2.328.0

# Via environment variable (recommended)
export GITHUB_TOKEN="ghp_xxxxxxxxxxxx"
github-release-version-checker -c 2.328.0
```

### Example 7: Bypass Cache

Force fresh API query:

```bash
github-release-version-checker -c 2.328.0 --no-cache
```

## Integration Patterns

### Shell Scripts

```bash
#!/bin/bash
set -e

VERSION=$(cat /opt/actions-runner/.runner | jq -r '.agentVersion')
OUTPUT=$(github-release-version-checker -c "$VERSION" --json)
STATUS=$(echo "$OUTPUT" | jq -r '.status')

if [ "$STATUS" = "expired" ]; then
 echo " Runner version is expired! Please update immediately."
 exit 1
elif [ "$STATUS" = "critical" ]; then
 echo " Runner version is critical. Update soon."
 exit 0
else
 echo " Runner version is current."
fi
```

### Monitoring (Prometheus)

Export metrics for Prometheus node_exporter textfile collector:

```bash
#!/bin/bash
OUTPUT=$(github-release-version-checker -c "$RUNNER_VERSION" --json)

cat > /var/lib/node_exporter/textfile/runner.prom <<EOF
# HELP runner_is_expired Whether the runner version is expired
# TYPE runner_is_expired gauge
runner_is_expired $(echo $OUTPUT | jq -r '.is_expired | if . then 1 else 0 end')

# HELP runner_releases_behind Number of releases behind
# TYPE runner_releases_behind gauge
runner_releases_behind $(echo $OUTPUT | jq -r '.releases_behind')

# HELP runner_days_since_update Days since a newer version was released
# TYPE runner_days_since_update gauge
runner_days_since_update $(echo $OUTPUT | jq -r '.days_since_update')
EOF
```

### CI/CD (Non-GitHub)

For Jenkins, GitLab CI, CircleCI, etc., use JSON output:

```bash
#!/bin/bash
OUTPUT=$(github-release-version-checker -c "$VERSION" --json)
IS_EXPIRED=$(echo "$OUTPUT" | jq -r '.is_expired')

if [ "$IS_EXPIRED" = "true" ]; then
 echo "Runner version check FAILED"
 exit 1
fi
```

## Status Codes

The CLI returns different exit codes based on the version status:

- `0`: Success (current, warning, or critical)
- `1`: Error (expired, version not found, or other error)

## Next Steps

- [GitHub Actions Integration](GITHUB-ACTIONS.md) - Use in CI/CD workflows
- [Library Usage](LIBRARY-USAGE.md) - Import as a Go library
- [Installation Guide](INSTALLATION.md) - Install the tool
