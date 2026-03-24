package github

import (
	"fmt"
	"strings"

	gogithub "github.com/google/go-github/v63/github"

	"github.com/Flashgap/marvin/pkg/github"
)

// TitleOrDescriptionWithPRNumber add the PR reference "(#PRNumber)" to the commit title or description if not already there.
// It first attempts to add it to the PR title, if the title exceeds the limit, it adds it to the description instead
func TitleOrDescriptionWithPRNumber(commitTitle, commitDescription string, prNumber int) (string, string) {
	prRef := fmt.Sprintf("#%d", prNumber)
	if strings.Contains(commitTitle, prRef) || strings.Contains(commitDescription, prRef) {
		return commitTitle, commitDescription
	}

	if len(commitTitle)+len(prRef)+1 < CommitTitleSizeLimit {
		return fmt.Sprintf("%s %s", commitTitle, prRef), commitDescription
	}

	return commitTitle, fmt.Sprintf("%s\n%s", commitDescription, prRef)
}

// ExtractPatchModifications returns a string containing all modifications for the given git patch string.
// The returned string doesn't contain the "+" character from the patch
func ExtractPatchModifications(patch string) string {
	lines := strings.Split(patch, "\n")
	var modifications strings.Builder

	for _, line := range lines {
		if strings.HasPrefix(line, "+") {
			_, _ = modifications.WriteString(line[1:])
			_, _ = modifications.WriteString("\n")
		}
	}

	return modifications.String()
}

// IsWorkInProgress returns true whether the PR is a draft PR or has the "Work In Progress" label
func IsWorkInProgress(pr *gogithub.PullRequest) bool {
	if github.IsLabelInList(pr.Labels, LabelWorkInProgress) {
		return true
	}

	return pr.GetDraft()
}
