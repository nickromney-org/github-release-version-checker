# GitHub Release Version Checker

![Build Status](https://github.com/nickromney-org/github-release-version-checker/workflows/CI/badge.svg)
![Go Version](https://img.shields.io/github/go-mod/go-version/nickromney-org/github-release-version-checker)
![License](https://img.shields.io/github/license/nickromney-org/github-release-version-checker)

A fast, flexible CLI tool and Go library for checking GitHub release versions against configurable expiry policies. Track version compliance for GitHub Actions runners, Kubernetes, Node.js, and any GitHub repository.

## Overview

The GitHub Release Version Checker helps you stay current with software releases by comparing your installed versions against the latest releases and alerting you when updates are needed. It supports both time-based policies (e.g., 30-day runner compliance) and version-based policies (e.g., Kubernetes N-3 support).

**Key Features:**

- **Multi-Repository Support**: Check versions for any GitHub repository
- **Local-First Workflow Auditing**: Scan checked-out repositories without a token
- **Flexible Policies**: Time-based (days) or semantic versioning-based (versions behind)
- **Multiple Output Formats**: Terminal (colourised), JSON (automation), CI (GitHub Actions)
- **Actions Supply-Chain Visibility**: Audit GitHub Actions refs, reusable workflows, and container images
- **Fast & Lightweight**: Single binary, ~10ms startup, no dependencies
- **Embedded Cache**: Minimizes API calls with intelligent release caching
- **Public API**: Import as a Go library in your own applications

<!-- version-check-start -->
## Daily Version Checks

**Last updated:** 15 Apr 2026 10:03 UTC

| Repository | Status | Latest Version | Command |
|------------|--------|----------------|---------|
| [GitHub Actions Runner](https://github.com/actions/runner/releases/tag/v2.333.1) | ![Status](https://img.shields.io/badge/current-green) | `v2.333.1` | `github-release-version-checker` |
| [Terraform](https://github.com/hashicorp/terraform/releases/tag/v1.14.8) | ![Status](https://img.shields.io/badge/current-green) | `v1.14.8` | `github-release-version-checker --repo hashicorp/terraform` |
| [Node.js](https://github.com/nodejs/node/releases/tag/v25.9.0) | ![Status](https://img.shields.io/badge/current-green) | `v25.9.0` | `github-release-version-checker --repo node` |

### GitHub Actions Runner Release Timeline

```text
2.333.1


📅 Release Expiry Timeline
─────────────────────────────────────────────────────
Version    Release Date   Expiry Date    Status
2.331.0    09 Jan 2026    27 Mar 2026    ❌ Expired 18 days ago
2.332.0    25 Feb 2026    17 Apr 2026    ✅ Valid (2 days left)
2.333.0    18 Mar 2026    26 Apr 2026    ✅ Valid (11 days left)
2.333.1    27 Mar 2026    -              ✅ Latest (18 days ago)

Checked at: 15 Apr 2026 10:03:49 UTC
```

## Quick Start

### Installation

Download the latest binary for your platform:

**macOS (Apple Silicon):**

```bash
curl -L -o github-release-version-checker https://github.com/nickromney-org/github-release-version-checker/releases/latest/download/github-release-version-checker-darwin-arm64
chmod +x github-release-version-checker
sudo mv github-release-version-checker /usr/local/bin/
```

**Linux (amd64):**

```bash
curl -L -o github-release-version-checker https://github.com/nickromney-org/github-release-version-checker/releases/latest/download/github-release-version-checker-linux-amd64
chmod +x github-release-version-checker
sudo mv github-release-version-checker /usr/local/bin/
```

See [docs/INSTALLATION.md](docs/INSTALLATION.md) for more installation options including Windows, Docker, and building from source.

### Basic Usage

**Check latest version:**

```bash
$ github-release-version-checker
2.329.0
```

**Compare your version:**

```bash
$ github-release-version-checker -c 2.328.0
2.329.0

 Version 2.328.0 (13 Aug 2025) expires 12 Sep 2025: Update to v2.329.0

 Release Expiry Timeline
─────────────────────────────────────────────────────
Version    Release Date    Expiry Date    Status
2.328.0    13 Aug 2025     12 Sep 2025    Valid (9 days left) ← Your version
2.329.0    14 Oct 2025     -              Latest
```

**Check other repositories:**

```bash
# Kubernetes
github-release-version-checker --repo kubernetes/kubernetes -c 1.28.0

# Node.js
github-release-version-checker --repo nodejs/node -c v20.0.0

# Terraform
github-release-version-checker --repo hashicorp/terraform -c 1.5.0

# Any repository
github-release-version-checker --repo owner/repo -c 1.0.0
```

**JSON output for automation:**

```bash
github-release-version-checker -c 2.328.0 --json
```

**GitHub Actions integration:**

```bash
github-release-version-checker -c 2.328.0 --ci
```

**Audit checked-out repositories without a token:**

```bash
# Scan the current repository
github-release-version-checker audit-workflows local --path .

# Scan a workspace and only include backend API repos
github-release-version-checker audit-workflows local --path ~/src --repo-filter 'backend-api-*'

# Return machine-readable JSON and only show floating refs
github-release-version-checker audit-workflows local --path ~/src --only-floating --output json

# By default, only consider upstream versions at least 7 days old
github-release-version-checker audit-workflows local --path ~/src

# Disable the cooldown and use the immediate latest version
github-release-version-checker audit-workflows local --path ~/src --cooldown 0

# Resolve the latest upstream SHA and show ready-to-paste pinned refs
github-release-version-checker audit-workflows local --path ~/src --pin-sha --view occurrences

# Show one row per usage with absolute workflow paths that terminals can open directly
github-release-version-checker audit-workflows local --path ~/src --view occurrences
```

**Audit remote repositories or owner boundaries:**

```bash
# Single repository
github-release-version-checker audit-workflows repo --repo owner/repo

# Entire owner boundary (user or organisation)
github-release-version-checker audit-workflows owner my-org --repo-filter 'backend-api-*' --output csv

# org remains available for organisation-specific usage
github-release-version-checker audit-workflows org my-org

# User owners include public repos by default. If your token belongs to that owner,
# private owned repos are included too.
github-release-version-checker audit-workflows owner nickromney --repo-filter 'private-*'
```

## Use Cases

### GitHub Actions Supply-Chain Audits

Audit GitHub Actions references for floating refs such as `@main`, `@master`, and `@latest`:

```bash
github-release-version-checker audit-workflows local --path . --only-floating
github-release-version-checker audit-workflows owner my-org --only-floating --fail-on floating
```

Audit workflow container images separately:

```bash
github-release-version-checker audit-containers local --path . --only-floating
```

The audit command reports:

- step `uses:` action references
- reusable workflow `jobs.<job>.uses`
- `docker://` action references

By default, latest-version resolution uses a 7-day cooldown similar to Dependabot. Use `--cooldown 0` to disable that delay.

`LATEST` means the highest eligible upstream release or tag after applying the cooldown. It is not constrained to the current major line already in use.

Use `--pin-sha` to resolve the latest upstream commit SHA for each action and show a copy-pasteable pinned `uses:` value in occurrence output.

Use `LATEST AGE` to see how many days old the selected upstream release is.

Use `--view summary` to aggregate versions in use, or `--view occurrences` to list every file, job, step, and line.

In local `--view occurrences` output, workflow paths are rendered as absolute filesystem paths so terminals and IDEs can open them directly.

## Shell Completions

The CLI exposes Cobra shell completions via the built-in `completion` command:

```bash
# zsh
github-release-version-checker completion zsh > ~/.zsh/completions/_github-release-version-checker

# bash
github-release-version-checker completion bash > ~/.local/share/bash-completion/completions/github-release-version-checker

# fish
github-release-version-checker completion fish > ~/.config/fish/completions/github-release-version-checker.fish
```

Use `github-release-version-checker completion --help` for the full shell list and install patterns.

## Agent Skill

The repository bundles a Codex skill at `skills/use-github-release-version-checker/`.

The skill teaches agents to:

- prefer explicit subcommands such as `check`, `audit-workflows`, and `audit-containers`
- default to local-first workflow audits when a token is not necessary
- use `-o json` for machine-readable output and `--view occurrences -o markdown` for clickable file locations
- use `owner` for remote bulk scans, with user-owner private repos only when authenticated as that same owner
- treat `audit-workflows` and `audit-containers` as separate surfaces

To install it into a local Codex skills directory:

```bash
mkdir -p "${CODEX_HOME:-$HOME/.codex}/skills"
cp -R skills/use-github-release-version-checker "${CODEX_HOME:-$HOME/.codex}/skills/"
```

### Self-Hosted Runner Compliance

GitHub requires self-hosted runners to update within 30 days. Check your runner version and get alerts:

```bash
VERSION=$(cat $RUNNER_HOME/.runner | jq -r '.agentVersion')
github-release-version-checker -c "$VERSION"
```

**Exit codes:**

- `0`: Current, warning, or critical (within policy)
- `1`: Expired (beyond 30 days)

### Kubernetes Version Tracking

Track Kubernetes versions against the N-3 minor version support policy:

```bash
github-release-version-checker --repo k8s -c 1.28.0
```

### CI/CD Integration

Integrate into GitHub Actions workflows with collapsible sections, annotations, and job summaries:

```yaml
- name: Check runner version
  run: |
    VERSION=$(cat $RUNNER_HOME/.runner | jq -r '.agentVersion')
    github-release-version-checker -c "$VERSION" --ci
```

### Go Library

Import and use in your own Go applications:

```go
import (
    "github.com/nickromney-org/github-release-version-checker/pkg/checker"
    "github.com/nickromney-org/github-release-version-checker/pkg/client"
    "github.com/nickromney-org/github-release-version-checker/pkg/policy"
)

ghClient := client.NewClient(token, "actions", "runner")
pol := policy.NewDaysPolicy(12, 30)
versionChecker := checker.NewCheckerWithPolicy(ghClient, checker.Config{}, pol)
analysis, err := versionChecker.Analyse(ctx, "2.328.0")
```

## Documentation

- **[Installation Guide](docs/INSTALLATION.md)** - Installation methods for all platforms
- **[CLI Usage Guide](docs/CLI-USAGE.md)** - Complete CLI reference with examples
- **[Library Usage Guide](docs/LIBRARY-USAGE.md)** - Go library API documentation
- **[GitHub Actions Integration](docs/GITHUB-ACTIONS.md)** - CI/CD integration examples
- **[Development Guide](docs/DEVELOPMENT.md)** - Building, testing, and contributing

## Supported Repositories

The tool includes predefined configurations for popular repositories:

| Repository | Alias | Policy Type | Threshold |
|------------|-------|-------------|-----------|
| actions/runner | `runner` | Days | 30 days |
| kubernetes/kubernetes | `k8s` | Versions | 3 minor versions |
| nodejs/node | `node` | Versions | 3 major versions |
| pulumi/pulumi | `pulumi` | Versions | 3 minor versions |
| hashicorp/terraform | - | Versions | 3 minor versions |
| alexellis/arkade | `arkade` | Versions | 3 minor versions |

You can check any GitHub repository using the `owner/repo` format or a GitHub URL.

## Policy Types

### Days-Based Policy

Time-based expiry for compliance requirements (e.g., GitHub Actions runners):

- **Warning**: New version available
- **Critical**: Within critical age window (default 12-30 days)
- **Expired**: Beyond maximum age threshold (default 30 days)

**Use cases:** Security patches, runner compliance, time-sensitive updates

### Version-Based Policy

Semantic versioning support windows (e.g., Kubernetes N-3):

- **Warning**: Behind but within support window
- **Critical**: Approaching end of support window
- **Expired**: Beyond support window (e.g., 4+ minor versions behind)

**Use cases:** Kubernetes, Node.js, libraries with semantic versioning

## Features

### Intelligent Caching

Embedded release cache minimizes API calls:

- 1 API call for current check (vs. 2 without cache)
- Daily automated cache updates
- Supports custom cache paths per repository
- Graceful fallback if cache is stale

### Output Formats

Three output modes for different use cases:

1. **Terminal**: Human-readable with colours and tables
2. **JSON**: Machine-readable for automation and monitoring
3. **CI**: GitHub Actions annotations and job summaries

The audit commands also support:

1. **Table**: Summary or occurrence-oriented dependency reports
2. **JSON**: Structured scan metadata, summary rows, and occurrences
3. **CSV**: Spreadsheet-friendly export for org-wide reporting

### Rate Limiting

Use `GITHUB_TOKEN` environment variable to increase API rate limits:

- Unauthenticated: 60 requests/hour
- Authenticated: 5,000 requests/hour

```bash
export GITHUB_TOKEN="ghp_xxxxxxxxxxxx"
github-release-version-checker -c 2.328.0
```

## Contributing

Contributions are welcome! Please see the [Development Guide](docs/DEVELOPMENT.md) for details on:

- Setting up your development environment
- Building and testing
- Code style guidelines (British English spelling)
- Submitting pull requests

**Quick start:**

```bash
# Clone the repository
git clone https://github.com/nickromney-org/github-release-version-checker.git
cd github-release-version-checker

# Build
make build

# Run tests
make test

# Format and lint
make fmt
make lint
```

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Resources

- **Repository**: <https://github.com/nickromney-org/github-release-version-checker>
- **Issues**: <https://github.com/nickromney-org/github-release-version-checker/issues>
- **API Documentation**: <https://pkg.go.dev/github.com/nickromney-org/github-release-version-checker>

## Acknowledgements

Built with:

- [spf13/cobra](https://github.com/spf13/cobra) - CLI framework
- [Masterminds/semver](https://github.com/Masterminds/semver) - Semantic versioning
- [google/go-github](https://github.com/google/go-github) - GitHub API client
- [fatih/color](https://github.com/fatih/color) - Terminal colours
