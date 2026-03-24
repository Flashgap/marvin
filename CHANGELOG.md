# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

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
