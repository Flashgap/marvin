package marvin

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	gogithub "github.com/google/go-github/v63/github"

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

// LegacyConfig carries the deprecated MARVIN_REPOSITORIES / MARVIN_REVIEWERS_TEAMS env vars.
// It has no effect on Marvin's actual behavior — which is driven entirely by each repository's
// own .marvin.yaml — and is used only to seed the one-time migration PR in attemptConfigMigration.
type LegacyConfig struct {
	Repositories   map[string]string
	ReviewersTeams map[string]string
}

// attemptConfigMigration opens a one-shot PR proposing a .marvin.yaml generated from this
// repository's legacy MARVIN_REPOSITORIES/MARVIN_REVIEWERS_TEAMS entry, if it has one. It is a
// no-op for repositories with no legacy entry, and best-effort: failures are logged, never
// returned, since this is a migration aid and must never block normal webhook processing.
func (s *service) attemptConfigMigration(ctx context.Context, webhook pkggithub.RepoSenderGetter) {
	log := middlewares.LoggerFromGHContext(ctx, "marvin.attemptConfigMigration")

	repoName := webhook.GetRepo().GetName()
	featuresStr, ok := s.legacyConfig.Repositories[repoName]
	if !ok {
		return
	}

	features := strings.Split(featuresStr, ";")
	team := s.legacyConfig.ReviewersTeams[repoName]
	defaultBranch := webhook.GetRepo().GetDefaultBranch()

	baseRef, _, err := s.githubService.GetRef(ctx, webhook, "refs/heads/"+defaultBranch)
	if err != nil {
		log.Errorf("config migration: cannot get default branch ref: %v", err)
		return
	}

	_, res, err := s.githubService.CreateRef(ctx, webhook, &gogithub.Reference{
		Ref:    new("refs/heads/" + migrationBranch),
		Object: &gogithub.GitObject{SHA: baseRef.GetObject().SHA},
	})
	if err != nil {
		if res != nil && res.StatusCode == http.StatusUnprocessableEntity {
			log.Infof("config migration branch %q already exists, already proposed once", migrationBranch)
			return
		}
		log.Errorf("config migration: cannot create branch %q: %v", migrationBranch, err)
		return
	}

	content := renderMigratedConfig(features, team)

	if _, _, err := s.githubService.CreateFile(ctx, webhook, RepoConfigFileName, &gogithub.RepositoryContentFileOptions{
		Message: new(fmt.Sprintf("Add %s (migrated from MARVIN_REPOSITORIES)", RepoConfigFileName)),
		Content: []byte(content),
		Branch:  new(migrationBranch),
	}); err != nil {
		log.Errorf("config migration: cannot create %s on %q: %v", RepoConfigFileName, migrationBranch, err)
		return
	}

	pr, _, err := s.githubService.CreatePullRequest(ctx, webhook, &gogithub.NewPullRequest{
		Title: new(fmt.Sprintf("Add %s", RepoConfigFileName)),
		Head:  new(migrationBranch),
		Base:  new(defaultBranch),
		Body:  new(migrationPRBody),
	})
	if err != nil {
		log.Errorf("config migration: cannot open PR: %v", err)
		return
	}

	log.Infof("opened config migration PR #%d", pr.GetNumber())
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
