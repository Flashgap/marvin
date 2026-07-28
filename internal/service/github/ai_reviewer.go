package github

import "strings"

// IsAIReviewerLogin returns true if login exactly matches, case-insensitively, one of the given
// aiLogins. Matching is exact (not substring) so human logins that happen to contain a bot name
// (e.g. "clement-copilot") are never misidentified.
func IsAIReviewerLogin(login string, aiLogins []string) bool {
	if login == "" {
		return false
	}

	login = strings.ToLower(login)

	for _, known := range aiLogins {
		if known != "" && login == strings.ToLower(known) {
			return true
		}
	}

	return false
}
