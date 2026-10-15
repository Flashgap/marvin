package reviewload

import (
	"context"

	gogithub "github.com/google/go-github/v90/github"

	"github.com/Flashgap/marvin/internal/middlewares"
	pkggithub "github.com/Flashgap/marvin/pkg/github"
	"github.com/Flashgap/marvin/pkg/linguist"
)

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
