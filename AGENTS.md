# Marvin

GitHub App that automates PR hygiene: validates titles/descriptions/Linear links,
auto-assigns reviewers, updates metadata, and merges PRs. 
Configurable per repository via environment variables.

## Stack

- Go 1.26+, Gin (HTTP), Ginkgo/Gomega (tests)
- Integrations: GitHub App, Linear, Slack, Jira
- Deploy: Cloud Run / Docker

## Structure

```
cmd/marvin/         # entry point
internal/
  config/           # env-var config parsing
  service/          # core business logic
  validate/         # PR validation rules
  route/            # API routes
pkg/
  github/           # GitHub API client
  linear/           # Linear API client
  slack/            # Slack client
  jira/             # Jira client
```

## Common commands

```bash
make build          # build binary
make test           # run tests (Ginkgo)
make mockgen        # regenerate mocks (required after changing Client interfaces)
golangci-lint run   # lint
```

## Key conventions

- All third-party clients must have mock-based tests
- Features are opt-in per repo via `MARVIN_REPOSITORIES` env var
- Config is entirely env-var based (see `config/local/marvin.env` for local dev)
- Branch names: `username/short-description`
- One logical change per PR
