---
name: use-github-release-version-checker
description: Check GitHub release policy status and audit GitHub Actions workflow dependencies with the `github-release-version-checker` CLI. Use when an agent needs explicit, non-interactive commands to compare versions against expiry policies, audit GitHub Action refs or workflow containers, inspect upstream latest versions and ages, or produce JSON or Markdown output for further tooling or review.
---

# Use GitHub Release Version Checker

Use `github-release-version-checker` as the primary interface for release-policy checks and GitHub Actions dependency audits. Prefer explicit subcommands and structured output.

## Quick Start

In this repository, prefer the built binary if it exists:

```bash
./bin/github-release-version-checker <subcommand> [args...]
```

If the binary has not been built yet, `go run .` is acceptable for one-off checks.

For agent workflows:

- Use `check` explicitly for version-policy checks.
- Use `audit-workflows` for action and reusable-workflow refs.
- Use `audit-containers` for job and service container images.
- Use `-o json` for machine-readable output.
- Use `--view occurrences -o markdown` when a human should be able to click file paths and upstream links.

Do not rely on the root no-argument behaviour in automation. Use an explicit subcommand every time.

## Read First

Start with the least broad command that answers the question.

- Check one release policy: `github-release-version-checker check --repo actions/runner -c 2.328.0`
- Audit one local repo: `github-release-version-checker audit-workflows local --path .`
- Audit local usage locations: `github-release-version-checker audit-workflows local --path . --view occurrences -o markdown`
- Audit floating refs only: `github-release-version-checker audit-workflows local --path . --only-floating`
- Audit containers only: `github-release-version-checker audit-containers local --path .`
- Audit a remote owner boundary: `github-release-version-checker audit-workflows owner nickromney-org -o json`

Use local scans first when the repository is already checked out. That avoids token concerns and usually gives the fastest path to the answer.

## Interpret Audit Output Correctly

- `CURRENT` is what the workflow currently references.
- `LATEST` is the highest eligible upstream release or tag after applying `--cooldown`.
- `LATEST AGE` is the age in days of that selected upstream ref.
- `--cooldown` defaults to `7 days`; use `--cooldown 0` to disable it.
- `--pin-sha` resolves the latest upstream commit SHA and adds copy-pasteable pinned refs.
- `--view summary` is dense inventory.
- `--view occurrences` is file-by-file detail.

For local occurrence output, workflow paths are rendered as absolute filesystem paths. In Markdown and JSON output, upstream source links are included so a human can verify the reported latest ref.

## Work In Agent Mode

Follow this decision order:

1. If the task is a version-policy question, use `check`.
2. If the task is about Actions usage on disk, use `audit-workflows local`.
3. If the task is about containers in workflows, use `audit-containers`.
4. If the task needs remote bulk scanning, prefer `owner` over `org`.
5. Use `--match` and `--repo-filter` before scanning very large workspaces or owners.

Prefer these patterns:

- Inventory for further processing: `-o json`
- Human review with links: `--view occurrences -o markdown`
- Narrow large scans: `--match TEXT --repo-filter 'backend-*'`
- Supply-chain triage: `--only-floating`
- SHA remediation prep: `--pin-sha --view occurrences`

## Remote Owner Semantics

- `owner` means either a GitHub user or a GitHub organisation.
- User owners expose public repos by default.
- User-owner private repos require authentication as that same user owner.
- Organisation owners return whatever repos the token can see.
- `org` remains available, but prefer `owner` for new remote bulk scans.

## Troubleshoot

Use these checks when output is surprising:

- Re-run with `--view occurrences` to see exact workflow locations.
- Re-run with `-o json` to inspect raw fields and upstream URLs.
- Use `--resolve-latest=false` if you only need a raw usage inventory.
- Use `--cooldown 0` if a very recent release should be considered immediately.
- If a workflow file is malformed, the scan warns and skips that file rather than aborting the whole run.
