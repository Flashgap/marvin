package github

import (
	"strings"

	"github.com/google/go-github/v63/github"
)

// ConsumePaginatedResource is a generic function to consume any paginated resource from the GitHub SDK
func ConsumePaginatedResource(perPage int, handler func(*github.ListOptions) (resp *github.Response, goToNextPage bool, err error)) error {
	// start with page 1 and the desired per-page limit
	opts := &github.ListOptions{
		Page:    1,
		PerPage: perPage,
	}

	for {
		resp, goToNextPage, err := handler(opts)
		if err != nil {
			return err
		}

		// Consumer wants to stop, exit the loop
		if !goToNextPage {
			break
		}

		// No more pages to fetch, exit the loop
		if resp == nil || resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	return nil
}

// IsLabel returns true if the given labelName matches the GitHub Label name by case-insensitive prefix
func IsLabel(ghLabel *github.Label, labelName string) bool {
	if ghLabel == nil {
		return false
	}

	return strings.HasPrefix(
		strings.ToLower(ghLabel.GetName()),
		strings.ToLower(labelName))
}

// IsLabelInList returns true is the given labelName finds a match by case-insensitive prefix in the slice of GitHub labels
func IsLabelInList(labels []*github.Label, labelName string) bool {
	return GetLabelInList(labels, labelName) != nil
}

// GetLabelInList returns a GitHub label from a list if it matches the given labelName by case-insensitive prefix
func GetLabelInList(labels []*github.Label, labelName string) *github.Label {
	for _, label := range labels {
		if IsLabel(label, labelName) {
			return label
		}
	}

	return nil
}
