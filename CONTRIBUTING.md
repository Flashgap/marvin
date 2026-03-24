# Contributing to Marvin

Thank you for taking the time to contribute! This document explains how to get the project running locally and how to submit changes.

## Table of contents

- [Getting started](#getting-started)
- [Making changes](#making-changes)
- [Submitting a pull request](#submitting-a-pull-request)
- [Code style](#code-style)

---

## Getting started

**Prerequisites:**
- Go 1.26+
- A GitHub App for local testing (see [README — GitHub App setup](README.md#github-app-setup))

**Clone and install:**

```bash
git clone https://github.com/Flashgap/marvin.git
cd marvin
go mod download
```

**Configure your local environment:**

```bash
cp config/local/marvin.env .env
# Edit .env with your own values
```

**Run:**

```bash
go run ./cmd/marvin
```

**Run tests:**

```bash
make test
```

**Regenerate mocks** (required after changing any `//go:generate` interface):

```bash
make mockgen
```

---

## Making changes

1. **Fork** the repository and create a branch from `main`:

   ```bash
   git checkout -b your-github-username/short-description
   ```

2. **Make your changes.** Keep PRs focused — one logical change per PR.

3. **Add or update tests.** Marvin uses [Ginkgo](https://onsi.github.io/ginkgo/) and [Gomega](https://onsi.github.io/gomega/). Run the suite with `make test`.

4. **If you changed a `Client` interface**, regenerate the mocks:

   ```bash
   make mockgen
   ```

5. **Lint your code.** The CI runs `golangci-lint`. You can run it locally:

   ```bash
   golangci-lint run
   ```

---

## Submitting a pull request

- Open a PR against the `main` branch.
- Fill in the PR template.
- Make sure CI is green before requesting a review.
- A maintainer will review and merge your PR.

---

## Code style

- Follow standard Go conventions (`gofmt`, `goimports`).
- Keep functions small and focused.
- Avoid adding new dependencies without discussion.
- New features that interact with third-party APIs (GitHub, Linear, Jira, Slack) must have mock-based tests.
