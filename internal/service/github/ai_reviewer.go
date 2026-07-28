package github

import "strings"

// defaultAIReviewerLogins lists known AI code-review bot logins recognized out of the box by
// require_ai_review, using their exact GitHub review-author login. Repositories can extend this
// list with org-specific bots via the MARVIN_AI_REVIEWER_LOGINS env var (see
// config.Marvin.MarvinAIReviewerLogins).
var defaultAIReviewerLogins = []string{"coderabbitai[bot]", "graphite-app[bot]", "copilot-pull-request-reviewer[bot]"}

// IsAIReviewerLogin returns true if login exactly matches, case-insensitively, one of the default
// AI reviewer bot logins or one of the given extraLogins. Matching is exact (not substring) so
// human logins that happen to contain a bot name (e.g. "clement-copilot") are never misidentified.
func IsAIReviewerLogin(login string, extraLogins []string) bool {
	if login == "" {
		return false
	}

	login = strings.ToLower(login)

	knownLogins := make([]string, 0, len(defaultAIReviewerLogins)+len(extraLogins))
	knownLogins = append(knownLogins, defaultAIReviewerLogins...)
	knownLogins = append(knownLogins, extraLogins...)

	for _, known := range knownLogins {
		if known != "" && login == strings.ToLower(known) {
			return true
		}
	}

	return false
}
