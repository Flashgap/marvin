# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### ⚠ Breaking changes

- **Removed `MARVIN_REPOSITORIES` and `MARVIN_REVIEWERS_TEAMS` env vars.** Per-repository settings
  (which features are enabled, which team(s) review which paths) now live in a `.marvin.yaml` YAML
  file committed at the root of each monitored repository's default branch, instead of Marvin's
  own deployment env vars. **Every previously configured repository needs a `.marvin.yaml` added
  before upgrading, or Marvin goes silent (all features disabled) for that repository until one is
  added.** See the README's "Repository configuration (`.marvin.yaml`)" section.
- The `FindAndAssignReviewers` signature (internal `github.Service` interface) changed from a
  single `fromTeam string` to `fromTeams []string`, to support requesting reviewers from more than
  one team on a single PR.
- **Renamed `MARVIN_REPO_CONFIG_CACHE_TTL` to `MARVIN_REPO_CONFIG_POLL_INTERVAL`** (same default,
  `5m`). Its meaning changed from a per-repository cache max-age to the interval at which Marvin
  polls every installed repository's `.marvin.yaml` in the background — see "Changed" below.

### Added

- **Path-based reviewer teams**: `.marvin.yaml`'s `reviewers.rules` maps glob patterns (`**`
  supported, e.g. `go/**`, `py/**`) to GitHub teams. A PR requests reviewers from the union of
  every team whose pattern matches a changed file, with `reviewers.default_team` as a fallback —
  enabling per-team review ownership within a single monorepo.
- `.marvin.yaml` is always fetched from a repository's default branch, never from a PR's head, so a
  PR author cannot edit their own review rules inside the PR being reviewed.
- **Automatic config migration**: for a repository with no `.marvin.yaml` yet but a legacy
  `MARVIN_REPOSITORIES` entry, Marvin opens a one-time PR (branch `marvin/add-marvin-yaml-config`)
  proposing a `.marvin.yaml` generated from that legacy config. `MARVIN_REPOSITORIES` and
  `MARVIN_REVIEWERS_TEAMS` are kept as deprecated, optional env vars solely to seed this migration
  PR — they have no other effect and can be unset once every repo has been migrated.
- Per-repo `ai_review.reviewer_logins` / `ai_review.status_contexts` in `.marvin.yaml`, extending the
  org-wide `MARVIN_AI_REVIEWER_LOGINS` / `MARVIN_AI_REVIEW_STATUS_CONTEXTS` for that repository only.
- If a repository's `.marvin.yaml` fails to load (invalid YAML, or a GitHub API error), Marvin now
  keeps using the last known-good configuration instead of silently disabling itself, and comments
  on the pull request explaining what went wrong.

### Changed

- **`.marvin.yaml` is no longer fetched from the webhook request path.** A background poller
  discovers every repository the GitHub App is installed on and periodically re-reads each one's
  `.marvin.yaml` (every `MARVIN_REPO_CONFIG_POLL_INTERVAL`); webhook handling only ever reads from
  that cache. This removes the risk of a slow or unavailable GitHub API call delaying (or timing
  out) webhook delivery.

## [1.0.0] — 2026-03-24

### Added

- **`auto_merge`** — Squash-merges a PR when the *Merge 🚀* label is added and all checks pass. Commit title is taken from the PR title; commit body from the `## Description` bullet points.
- **`auto_review_assign`** — Requests reviewers from a configured team when the *Ready for review 👌* label is added, picking people by smallest current review load.
- **`auto_approve`** — Adds the *Approved* label once the required number of approvals is reached.
- **`auto_changes_required`** — Adds the *Changes required* label and notifies the PR author via Slack when changes are requested.
- **`auto_assignee`** — Assigns the PR opener as assignee when none is set.
- **`update_title`** — Corrects the PR title to the `ISSUE-ID: description` format and removes GitHub-generated branch noise.
- **`update_linear_link`** — Auto-fills the `## Fixed issues` section from the git branch name if it is empty.
- **`check_title`** — Validates that the PR title starts with a configured issue prefix (e.g. `ENG-123:`).
- **`check_description`** — Validates that the `## Description` section contains only bullet points.
- **`check_time_spent`** — Validates that the `## Time spent` section contains a valid float value.
- **`check_linear_link`** — Validates that a Linear issue URL is present and consistent with the title.
- **`check_linear_project`** — Validates that the linked Linear issue belongs to a project.
- **`check_changelog`** — Validates that `CHANGELOG.md` was updated and references the PR number.
- **`slack_notify`** — Sends a Slack DM to a reviewer when they are assigned.
- **`auto_cap_report`** — On merge, creates a Jira task from the Linear issue for capitalization reporting.
- Configurable Linear workspace slug (`LINEAR_WORKSPACE_SLUG`) and issue prefixes (`LINEAR_ISSUE_PREFIXES`).
- Docker image published to GitHub Container Registry (`ghcr.io/flashgap/marvin`).
