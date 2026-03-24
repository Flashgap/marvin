package github

import (
	"bufio"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	descriptionSectionRegex = regexp.MustCompile(`(?s)##\s*Description(.*?)(?:##|$)`)
	timeSpentSectionRegex   = regexp.MustCompile(`(?s)##\s*Time spent(.*?)(?:##|$)`)
	fixedIssueSectionRegex  = regexp.MustCompile(`(?s)##\s*Fixed issues(.*?)(?:##|$)`)

	timeSpentRegex = regexp.MustCompile(`(\d+[.,]?\d*) hour`)

	markdownLineRegex = regexp.MustCompile(`^(\s*[-+*])`)
	// Regex pattern to match Markdown checkboxes: [ ], [x], [X]
	removeMarkdownCheckboxesRegex = regexp.MustCompile(`\[([xX\s]?)\]\s*`)
	htmlCommentRegex              = regexp.MustCompile(`<!--[\s\S]*?-->`)
)

// PRParserConfig holds the runtime configuration needed to parse PR contents.
type PRParserConfig struct {
	// WorkspaceSlug is the Linear workspace slug (the path segment in issue URLs).
	// ex: "my-org" for https://linear.app/my-org/issue/ENG-123
	WorkspaceSlug string
	// IssuePrefixes is the list of issue shorthand prefixes used by the team.
	// ex: ["ENG", "APP", "BUG"]
	IssuePrefixes []string
}

func (c PRParserConfig) prefixAlternation() string {
	quoted := make([]string, len(c.IssuePrefixes))
	for i, p := range c.IssuePrefixes {
		quoted[i] = regexp.QuoteMeta(p)
	}
	return strings.Join(quoted, "|")
}

func (c PRParserConfig) titleIssueIDRegex() *regexp.Regexp {
	return regexp.MustCompile(fmt.Sprintf(`(?i)^(?:%s)(?:\s|-)\d+`, c.prefixAlternation()))
}

func (c PRParserConfig) branchIDRegex() *regexp.Regexp {
	return regexp.MustCompile(fmt.Sprintf(`(?i)(?:%s)(?:\s|-)\d+`, c.prefixAlternation()))
}

func (c PRParserConfig) cleanTitleRegex() *regexp.Regexp {
	return regexp.MustCompile(fmt.Sprintf(`(?i)(?:(\w+)\s*\/(?:%s)\s*\d+\s*)?`, c.prefixAlternation()))
}

func (c PRParserConfig) linearLinkRegex() *regexp.Regexp {
	return regexp.MustCompile(fmt.Sprintf(
		`(?i)https:\/\/linear\.app\/%s\/issue\/((%s)-\d+)`,
		regexp.QuoteMeta(c.WorkspaceSlug),
		c.prefixAlternation(),
	))
}

type PRInfo struct {
	// Title should be composed by the issue ref
	// ex: PRO-300: something to develop || ENG-300: something to develop
	Title string

	// CleanedTitle is the wanted PR title.
	// If that's not the same as Title, you need to update your title with this one.
	CleanedTitle string

	// CleanedBody is the wanted PR body.
	// For example, it could add the linear link created from the git branch.
	CleanedBody string

	// list of things done in the PR
	// should be a bullets points
	Description string

	// LinearLinkIssueID is the issue ID extracted from the linear link
	// ex: "ENG-387"
	LinearLinkIssueID string

	// Issue id extracted from the PR title.
	// ex: "ENG-387"
	// could be empty if not found
	TitleIssue string

	// BranchIssueID is the issue ID extracted from the branch name
	// ex: "ENG-387"
	// could be empty if not found
	BranchIssueID string

	// TimeSpent developing the PR.
	// ex: 1,5 hours
	TimeSpent float64

	AddTitleIssueID bool
	AddLinearLink   bool
}

// ParsePRContents returns a PRInfo from the PR's title body and branch name
func ParsePRContents(title string, body string, branchName string, cfg PRParserConfig) (PRInfo, error) {
	var errs error
	var newBody string
	var addTitleIssueID bool
	var addLinearLink bool

	newTitle := CleanTitle(title, cfg)
	body = RemoveHTMLComments(body)

	description, err := ExtractDescription(body)
	if err != nil {
		errs = errors.Join(errs, fmt.Errorf("cannot extract description: %w", err))
	}

	timeSpent, err := ExtractTimeSpent(body)
	if err != nil {
		errs = errors.Join(errs, fmt.Errorf("cannot extract time spent: %w", err))
	}

	linearLinkFromBranch, linearIssueIDFromBranch, err := LinearLinkFromBranch(branchName, cfg)
	if err != nil {
		errs = errors.Join(errs, fmt.Errorf("cannot extract issue id from branch: %w", err))
	}

	var linearIssueID string

	linearIssueIDFromLink, err := ExtractFixedIssue(body, cfg)
	if err != nil {
		errs = errors.Join(errs, fmt.Errorf("cannot extract linear link: %w", err))
		if !errors.Is(errs, ErrBranchFormat) {
			// There is no linear link, but it can be built from git branch.
			// Use the link built from git branch and add it to the PR.
			newBody, err = AddLinearLink(body, linearLinkFromBranch)
			if err != nil {
				errs = errors.Join(errs, fmt.Errorf("cannot add linear link to body: %w", err))
			} else {
				addLinearLink = true
			}
			linearIssueID = linearIssueIDFromBranch
		}
	} else {
		linearIssueID = linearIssueIDFromLink
	}

	titleIssueID, err := ExtractIssueFromTitle(title, cfg)
	if err != nil {
		errs = errors.Join(errs, err)
		if errors.Is(err, ErrIssueNotFoundInTitle) && (!errors.Is(errs, ErrLinearLink) || !errors.Is(errs, ErrBranchFormat)) {
			// add the issue ID reference to title
			newTitle = fmt.Sprintf("%s: %s", linearIssueID, newTitle)
			addTitleIssueID = true
		}
	}

	ids := make([]string, 0, 3)
	if titleIssueID != "" {
		ids = append(ids, titleIssueID)
	}
	if linearIssueIDFromBranch != "" {
		ids = append(ids, linearIssueIDFromBranch)
	}
	if linearIssueIDFromLink != "" {
		ids = append(ids, linearIssueIDFromLink)
	}

	if len(ids) > 1 {
		first := ids[0]

		for _, id := range ids[1:] {
			if !strings.EqualFold(first, id) {
				errs = errors.Join(errs, ErrInconsistentIssueID)
			}
		}
	}

	return PRInfo{
		Title:             title,
		CleanedTitle:      newTitle,
		CleanedBody:       newBody,
		Description:       description,
		TimeSpent:         timeSpent,
		LinearLinkIssueID: linearIssueIDFromLink,
		BranchIssueID:     linearIssueIDFromBranch,
		AddTitleIssueID:   addTitleIssueID,
		AddLinearLink:     addLinearLink,
	}, errs
}

// ValidateDescription validates a description string.
// A description is valid if it's composed by only bullet points.
// It can be a markdown todo.
func ValidateDescription(input string) bool {
	scanner := bufio.NewScanner(strings.NewReader(input))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// empty lines are accepted
		if line == "" {
			continue
		}

		if !markdownLineRegex.MatchString(line) {
			return false
		}
	}

	return true
}

// RemoveMarkdownCheckboxes replace matched checkboxes with an empty string
func RemoveMarkdownCheckboxes(input string) string {
	return removeMarkdownCheckboxesRegex.ReplaceAllString(input, "")
}

// ExtractDescription returns everything in the "## Description" part and validates it
func ExtractDescription(input string) (string, error) {
	match := descriptionSectionRegex.FindStringSubmatch(input)
	if len(match) < 2 {
		return "", fmt.Errorf("%w: %w", ErrSectionNotFound, ErrInvalidDescription)
	}

	description := strings.TrimSpace(match[1])
	if ok := ValidateDescription(description); !ok {
		return "", fmt.Errorf("description is not valid, should be composed only by bullet points: %w", ErrInvalidDescription)
	}

	return RemoveMarkdownCheckboxes(description), nil
}

// AddLinearLink returns the prBody with linearLink added to its dedicated section.
func AddLinearLink(prBody string, linearLink string) (string, error) {
	match := fixedIssueSectionRegex.FindStringSubmatch(prBody)
	if len(match) < 2 {
		return "", fmt.Errorf("%w: %w", ErrSectionNotFound, ErrLinearLink)
	}

	content := strings.TrimSpace(match[0])
	linearContent := strings.TrimSuffix(content, "##")
	// Remove placeholder
	newLinearContent := fmt.Sprintf("%s%s\n", linearContent, linearLink)
	newLinearContent = strings.ReplaceAll(newLinearContent, "LINEAR_LINK", "")
	newBody := strings.ReplaceAll(prBody, linearContent, newLinearContent)

	return newBody, nil
}

// ExtractFixedIssue extract the related issue id from linear link found in the prBody
func ExtractFixedIssue(prBody string, cfg PRParserConfig) (string, error) {
	match := fixedIssueSectionRegex.FindStringSubmatch(prBody)
	if len(match) < 2 {
		return "", fmt.Errorf("%w: %w", ErrSectionNotFound, ErrLinearLink)
	}

	content := strings.TrimSpace(match[1])
	match = cfg.linearLinkRegex().FindStringSubmatch(content)
	if len(match) == 0 {
		return "", fmt.Errorf("no linear link found: %w", ErrLinearLink)
	}

	return match[1], nil
}

// CleanTitle removes what's not wanted in the PR title,
// things that Github automatically adds based on the branch name
func CleanTitle(input string, cfg PRParserConfig) string {
	result := cfg.cleanTitleRegex().ReplaceAllString(input, "")
	return strings.TrimSpace(result)
}

// ExtractTimeSpent returns the time spent found from the prBody
func ExtractTimeSpent(prBody string) (float64, error) {
	match := timeSpentSectionRegex.FindStringSubmatch(prBody)
	if len(match) < 2 {
		return 0.0, fmt.Errorf("%w: %w", ErrSectionNotFound, ErrTimeSpent)
	}

	// extract time spent from "Time spent" section
	timeSpentBody := strings.TrimSpace(match[1])
	timeSpentMatch := timeSpentRegex.FindStringSubmatch(timeSpentBody)
	if len(timeSpentMatch) == 0 {
		return 0, fmt.Errorf("time spent is not digits: %w", ErrTimeSpent)
	}

	// replace "," with "." to ensure a valid float format
	// so 1.5 and 1,5 works for example
	hours := strings.ReplaceAll(timeSpentMatch[1], ",", ".")
	digit, err := strconv.ParseFloat(hours, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid digit format: %w", ErrTimeSpent)
	}

	if digit < MinTimeSpent {
		return 0, fmt.Errorf("time spent cannot less than %f: %w", MinTimeSpent, ErrTimeSpent)
	}

	return digit, nil
}

// ExtractIssueFromTitle returns the issue from the title, error if not found.
// expected format of title is: PREFIX-digits your title here
// returns "PREFIX-digits" as a string
func ExtractIssueFromTitle(title string, cfg PRParserConfig) (string, error) {
	title = strings.TrimSpace(title)
	match := cfg.titleIssueIDRegex().FindStringSubmatch(title)

	if len(match) == 0 {
		return "", ErrIssueNotFoundInTitle
	}

	return match[0], nil
}

// LinearLinkFromBranch returns the linear link and the issueID from git branch
func LinearLinkFromBranch(branch string, cfg PRParserConfig) (string, string, error) {
	match := cfg.branchIDRegex().FindStringSubmatch(branch)
	if len(match) == 0 {
		return "", "", ErrBranchFormat
	}

	issueID := strings.ToUpper(match[0])
	return fmt.Sprintf("https://linear.app/%s/issue/%s", cfg.WorkspaceSlug, issueID), issueID, nil
}

func RemoveHTMLComments(input string) string {
	return htmlCommentRegex.ReplaceAllString(input, "")
}
