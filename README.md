# GitHub Release Version Checker

![Build Status](https://github.com/nickromney-org/github-release-version-checker/workflows/CI/badge.svg)
![Go Version](https://img.shields.io/github/go-mod/go-version/nickromney-org/github-release-version-checker)
![License](https://img.shields.io/github/license/nickromney-org/github-release-version-checker)

A fast, flexible CLI tool and Go library for checking GitHub release versions against configurable expiry policies. Track version compliance for GitHub Actions runners, Kubernetes, Node.js, and any GitHub repository.

## Overview

The GitHub Release Version Checker helps you stay current with software releases by comparing your installed versions against the latest releases and alerting you when updates are needed. It supports both time-based policies (e.g., 30-day runner compliance) and version-based policies (e.g., Kubernetes N-3 support).

**Key Features:**

- **Multi-Repository Support**: Check versions for any GitHub repository
- **Flexible Policies**: Time-based (days) or semantic versioning-based (versions behind)
- **Multiple Output Formats**: Terminal (colourised), JSON (automation), CI (GitHub Actions)
- **Fast & Lightweight**: Single binary, ~10ms startup, no dependencies
- **Embedded Cache**: Minimizes API calls with intelligent release caching
- **Public API**: Import as a Go library in your own applications

<!-- version-check-start -->
## Daily Version Checks

**Last updated:** 07 Jan 2026 09:16 UTC

| Repository | Status | Latest Version | Command |
|------------|--------|----------------|---------|
| [GitHub Actions Runner](https://github.com/actions/runner/releases/tag/v2.330.0) | ![Status](https://img.shields.io/badge/current-green) | `v2.330.0` | `github-release-version-checker` |
| [Terraform](https://github.com/hashicorp/terraform/releases/tag/v1.14.3) | ![Status](https://img.shields.io/badge/current-green) | `v1.14.3` | `github-release-version-checker --repo hashicorp/terraform` |
| [Node.js](https://github.com/nodejs/node/releases/tag/v25.2.1) | ![Status](https://img.shields.io/badge/current-green) | `v25.2.1` | `github-release-version-checker --repo node` |

### GitHub Actions Runner Release Timeline

```text
2.330.0


📅 Release Expiry Timeline
─────────────────────────────────────────────────────
Version    Release Date   Expiry Date    Status
2.327.1    25 Jul 2025    12 Sep 2025    ❌ Expired 116 days ago
2.328.0    13 Aug 2025    13 Nov 2025    ❌ Expired 54 days ago
2.329.0    14 Oct 2025    19 Dec 2025    ❌ Expired 18 days ago
2.330.0    19 Nov 2025    -              ✅ Latest (48 days ago)

Checked at: 7 Jan 2026 09:16:32 UTC
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

## Use Cases

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
