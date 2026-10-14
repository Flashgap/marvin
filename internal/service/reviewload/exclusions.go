package reviewload

import (
	"context"
	"fmt"
	"path"

	gogithub "github.com/google/go-github/v90/github"

	"github.com/Flashgap/marvin/internal/middlewares"
	pkggithub "github.com/Flashgap/marvin/pkg/github"
	"github.com/Flashgap/marvin/pkg/linguist"
)

// exclusions returns what leaves a file out of review load: being marked linguist-generated or
// linguist-vendored by the .gitattributes files of the repository's default branch, never of a PR's head,
// which would let a PR author shrink their own PR's score. Reading them is best effort: it returns nil,
// counting every file as before, when they exclude nothing or can't be read, which never fails ranking.
func (s *service) exclusions(ctx context.Context, webhook pkggithub.RepoSenderGetter) *linguist.Matcher {
	log := middlewares.LoggerFromGHContext(ctx, "reviewload.exclusions")

	files, err := s.gitattributes(ctx, webhook)
	if err != nil {
		log.Warnf("cannot read the %s files of %s, counting generated and vendored files: %v", linguist.FileName, webhook.GetRepo().GetFullName(), err)
		return nil
	}

	matcher, err := linguist.NewMatcher(files)
	if err != nil {
		log.Warnf("invalid %s file in %s, counting generated and vendored files: %v", linguist.FileName, webhook.GetRepo().GetFullName(), err)
		return nil
	}

	return matcher
}

// gitattributes returns the content of every .gitattributes file of the repository's default branch, by
// path. One tree call lists them, their content is only fetched for blobs not seen before.
func (s *service) gitattributes(ctx context.Context, webhook pkggithub.RepoSenderGetter) (map[string]string, error) {
	branch := webhook.GetRepo().GetDefaultBranch()
	tree, _, err := s.githubClient.GetTree(ctx, webhook, branch, true)
	if err != nil {
		return nil, fmt.Errorf("getting the tree of %s: %w", branch, err)
	}
	if tree.GetTruncated() {
		log := middlewares.LoggerFromGHContext(ctx, "reviewload.gitattributes")
		log.Warnf("the tree of %s is truncated, only reading the %s files of its first %d entries", branch, linguist.FileName, len(tree.GetEntries()))
	}

	files := make(map[string]string)
	for _, entry := range tree.GetEntries() {
		if entry.GetType() != "blob" || path.Base(entry.GetPath()) != linguist.FileName {
			continue
		}

		content, err := s.blob(ctx, webhook, entry)
		if err != nil {
			return nil, fmt.Errorf("getting %s: %w", entry.GetPath(), err)
		}
		files[entry.GetPath()] = content
	}

	return files, nil
}

// blob returns the content of a .gitattributes file, cached by blob SHA. Parsing is left to each ranking:
// the same blob parses differently in each directory holding it.
func (s *service) blob(ctx context.Context, webhook pkggithub.RepoSenderGetter, entry *gogithub.TreeEntry) (string, error) {
	s.mu.Lock()
	content, ok := s.blobs[entry.GetSHA()]
	s.mu.Unlock()
	if ok {
		return content, nil
	}

	raw, _, err := s.githubClient.GetBlobRaw(ctx, webhook, entry.GetSHA())
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	s.blobs[entry.GetSHA()] = string(raw)
	s.mu.Unlock()

	return string(raw), nil
}

// additions returns the lines pr adds to the files excluded doesn't exclude, or to every file when excluded
// is nil. Failing to list the files of a PR changing more than ListOpenPRsWithReviewers lists counts all of
// them, as with a nil excluded.
func (s *service) additions(ctx context.Context, webhook pkggithub.RepoSenderGetter, pr pkggithub.OpenPRReviewLoad, excluded *linguist.Matcher) int {
	if excluded == nil {
		return pr.Additions
	}

	files := pr.Files
	if pr.MoreFiles {
		var err error
		if files, err = s.listPRFiles(ctx, webhook, pr.Number); err != nil {
			log := middlewares.LoggerFromGHContext(ctx, "reviewload.additions")
			log.Warnf("cannot list the files of PR #%d, counting its generated and vendored files: %v", pr.Number, err)
			return pr.Additions
		}
	}

	additions := 0
	for _, file := range files {
		if !excluded.Excludes(file.Path) {
			additions += file.Additions
		}
	}

	return additions
}

func (s *service) listPRFiles(ctx context.Context, webhook pkggithub.RepoSenderGetter, prNumber int) ([]pkggithub.FileAdditions, error) {
	var files []pkggithub.FileAdditions
	err := pkggithub.ConsumePaginatedResource(pkggithub.MaxPerPage, func(opts *gogithub.ListOptions) (*gogithub.Response, bool, error) {
		page, res, err := s.githubClient.ListPRFiles(ctx, webhook, prNumber, opts)
		if err != nil {
			return nil, false, err
		}

		for _, file := range page {
			files = append(files, pkggithub.FileAdditions{Path: file.GetFilename(), Additions: file.GetAdditions()})
		}

		return res, true, nil
	})

	return files, err
}
