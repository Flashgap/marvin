package marvin

import (
	"context"
	"fmt"
	"path"

	"github.com/Flashgap/logrus"
	gogithub "github.com/google/go-github/v90/github"

	pkggithub "github.com/Flashgap/marvin/pkg/github"
	"github.com/Flashgap/marvin/pkg/linguist"
)

func (p *repoConfigProvider) Gitattributes(repo *gogithub.Repository) *linguist.Matcher {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.gitattributes[repo.GetFullName()]
}

// pollGitattributes refreshes the cached matcher of the files a repository's .gitattributes files mark
// linguist-generated or linguist-vendored, whether or not it has a .marvin.yaml: review load ranks every
// installed repository. On failure it keeps the last matcher read, if any.
func (p *repoConfigProvider) pollGitattributes(ctx context.Context, repo *gogithub.Repository) {
	files, err := p.fetchGitattributes(ctx, installedRepoWebhook{repo: repo})
	if err != nil {
		logrus.Warnf("%s: failed to refresh its %s files (%v), using the last ones read, if any", repo.GetFullName(), linguist.FileName, err)
		return
	}

	matcher := linguist.NewMatcher(files)

	p.mu.Lock()
	p.gitattributes[repo.GetFullName()] = matcher
	p.mu.Unlock()
}

// fetchGitattributes returns the content of every .gitattributes file of the repository's default branch, by
// path. As with .marvin.yaml, they are never read off a PR's head, which would let a PR author shrink their own
// PR's review load. One tree call lists them, their content is only downloaded for blobs not seen before.
func (p *repoConfigProvider) fetchGitattributes(ctx context.Context, webhook pkggithub.RepoSenderGetter) (map[string]string, error) {
	branch := webhook.GetRepo().GetDefaultBranch()
	tree, _, err := p.githubClient.GetTree(ctx, webhook, branch, true)
	if err != nil {
		return nil, fmt.Errorf("getting the tree of %s: %w", branch, err)
	}
	if tree.GetTruncated() {
		// The .gitattributes files left out could change what those listed decide
		return nil, fmt.Errorf("the tree of %s is truncated after %d entries", branch, len(tree.GetEntries()))
	}

	files := make(map[string]string)
	for _, entry := range tree.GetEntries() {
		if entry.GetType() != "blob" || path.Base(entry.GetPath()) != linguist.FileName {
			continue
		}

		content, ok := p.blobs[entry.GetSHA()]
		if !ok {
			raw, _, err := p.githubClient.GetBlobRaw(ctx, webhook, entry.GetSHA())
			if err != nil {
				return nil, fmt.Errorf("getting %s: %w", entry.GetPath(), err)
			}
			content = string(raw)
			p.blobs[entry.GetSHA()] = content
		}
		files[entry.GetPath()] = content
	}

	return files, nil
}
