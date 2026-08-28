package marvin

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/Flashgap/logrus"
	gogithub "github.com/google/go-github/v63/github"

	"github.com/Flashgap/marvin/internal/config"
	"github.com/Flashgap/marvin/internal/middlewares"
	pkggithub "github.com/Flashgap/marvin/pkg/github"
)

// migrationBranch is the fixed branch name used for the .marvin.yaml migration PR. Reusing the
// same name on every attempt is what makes the migration idempotent: creating the ref a second
// time fails with "already exists" (422), so Marvin only ever proposes it once per repository,
// regardless of whether that first PR is still open, merged, or was closed without merging.
const migrationBranch = "marvin/add-marvin-yaml-config"

const migrationPRBody = `This PR was opened automatically by Marvin.

Marvin v2.0.0 moved per-repository settings out of its own deployment env vars and into a ` + "`" + RepoConfigFileName + "`" + ` file
committed in each repository it manages. This repository previously had an entry in the
(now deprecated) ` + "`MARVIN_REPOSITORIES`" + ` / ` + "`MARVIN_REVIEWERS_TEAMS`" + ` env vars — this PR carries that
configuration forward as a starting point.

Review the generated file, adjust it as needed (e.g. add path-based reviewer rules for a
monorepo), and merge whenever you're ready.
`

// legacyFallbackConfig synthesizes a GitHubRepositoryConfiguration for a repository that has no
// .marvin.yaml yet but does have a legacy MARVIN_REPOSITORIES entry (orgConfig.MarvinLegacyRepositories),
// by rendering that entry through the same .marvin.yaml schema and parser used for a real file
// (renderMigratedConfig + parseRepoConfig) — so the fallback behaves exactly like the eventual
// migrated .marvin.yaml will. Returns nil if the repository has no legacy entry.
func legacyFallbackConfig(orgConfig config.Marvin, repoName string) *GitHubRepositoryConfiguration {
	features, team, ok := legacyEntry(orgConfig, repoName)
	if !ok {
		return nil
	}

	repoConfig, err := parseRepoConfig(renderMigratedConfig(features, team), orgConfig)
	if err != nil {
		// renderMigratedConfig always produces valid YAML; a failure here means Marvin itself has a
		// bug, not a data problem with the legacy entry.
		logrus.Errorf("legacy config fallback: failed to parse synthesized %s for %s: %v", RepoConfigFileName, repoName, err)
		return nil
	}

	return repoConfig
}

// legacyEntry looks up repoName's legacy MARVIN_REPOSITORIES/MARVIN_REVIEWERS_TEAMS entry, if any.
//
//nolint:staticcheck // SA1019: deprecated fields, this is their one intended remaining use
func legacyEntry(orgConfig config.Marvin, repoName string) (features []string, team string, ok bool) {
	featuresStr, ok := lookupTrimmed(orgConfig.MarvinLegacyRepositories, repoName)
	if !ok {
		return nil, "", false
	}

	team, _ = lookupTrimmed(orgConfig.MarvinLegacyReviewersTeams, repoName)

	return strings.Split(featuresStr, ";"), strings.TrimSpace(team), true
}

// lookupTrimmed returns m[repoName], matching keys on their whitespace-trimmed form.
func lookupTrimmed(m map[string]string, repoName string) (string, bool) {
	for key, value := range m {
		if strings.TrimSpace(key) == repoName {
			return value, true
		}
	}

	return "", false
}

// attemptConfigMigration opens a one-shot PR proposing a .marvin.yaml generated from this
// repository's legacy MARVIN_REPOSITORIES/MARVIN_REVIEWERS_TEAMS entry, if it has one. It is a
// no-op for repositories with no legacy entry, and best-effort: failures are logged, never
// returned, since this is a migration aid and must never block the poll loop.
func (p *repoConfigProvider) attemptConfigMigration(ctx context.Context, webhook pkggithub.RepoSenderGetter) {
	log := middlewares.LoggerFromGHContext(ctx, "marvin.attemptConfigMigration")
	repoName := webhook.GetRepo().GetName()
	features, team, ok := legacyEntry(p.orgConfig, repoName)
	if !ok {
		return
	}

	defaultBranch := webhook.GetRepo().GetDefaultBranch()

	baseRef, _, err := p.githubClient.GetRef(ctx, webhook, "refs/heads/"+defaultBranch)
	if err != nil {
		log.Errorf("config migration: cannot get default branch ref for %s: %v", repoName, err)
		return
	}

	_, res, err := p.githubClient.CreateRef(ctx, webhook, &gogithub.Reference{
		Ref:    new("refs/heads/" + migrationBranch),
		Object: &gogithub.GitObject{SHA: baseRef.GetObject().SHA},
	})
	if err != nil {
		if res != nil && res.StatusCode == http.StatusUnprocessableEntity {
			log.Infof("config migration branch %q already exists for %s, already proposed once", migrationBranch, repoName)
			return
		}
		log.Errorf("config migration: cannot create branch %q for %s: %v", migrationBranch, repoName, err)
		return
	}

	content := renderMigratedConfig(features, team)

	if _, _, err := p.githubClient.CreateFile(ctx, webhook, RepoConfigFileName, &gogithub.RepositoryContentFileOptions{
		Message: new(fmt.Sprintf("Add %s (migrated from MARVIN_REPOSITORIES)", RepoConfigFileName)),
		Content: []byte(content),
		Branch:  new(migrationBranch),
	}); err != nil {
		log.Errorf("config migration: cannot create %s on %q for %s: %v", RepoConfigFileName, migrationBranch, repoName, err)
		return
	}

	pr, _, err := p.githubClient.CreatePullRequest(ctx, webhook, &gogithub.NewPullRequest{
		Title: new(fmt.Sprintf("Add %s", RepoConfigFileName)),
		Head:  new(migrationBranch),
		Base:  new(defaultBranch),
		Body:  new(migrationPRBody),
	})
	if err != nil {
		log.Errorf("config migration: cannot open PR for %s: %v", repoName, err)
		return
	}

	log.Infof("opened config migration PR #%d for %s", pr.GetNumber(), repoName)
}

// renderMigratedConfig renders a .marvin.yaml body from a legacy MARVIN_REPOSITORIES feature list
// and MARVIN_REVIEWERS_TEAMS team (empty string if the repository had none).
func renderMigratedConfig(features []string, team string) string {
	var b strings.Builder

	b.WriteString("# " + RepoConfigFileName + "\n")
	b.WriteString("#\n")
	b.WriteString("# Auto-migrated by Marvin from the deprecated MARVIN_REPOSITORIES / MARVIN_REVIEWERS_TEAMS\n")
	b.WriteString("# env vars. Review and adjust as needed, then merge this PR.\n")
	b.WriteString("\n")

	b.WriteString("features:\n")
	for _, feature := range features {
		feature = strings.TrimSpace(feature)
		if feature == "" {
			continue
		}
		b.WriteString("  - " + feature + "\n")
	}

	if team != "" {
		b.WriteString("\n")
		b.WriteString("reviewers:\n")
		b.WriteString("  default_team: " + team + "\n")
	}

	return b.String()
}
