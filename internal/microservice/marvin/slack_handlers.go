package marvin

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/slack-go/slack"

	"github.com/Flashgap/marvin/internal/service/reviewload"
	weberrors "github.com/Flashgap/marvin/internal/web/errors"
	stderror "github.com/Flashgap/marvin/pkg/stderr"
)

func (ctrl *Controller) lockHandler(c *gin.Context) {
	if ctrl.lockService == nil {
		c.AbortWithStatusJSON(http.StatusNotImplemented, weberrors.GenericNotImplementedError)
		return
	}

	cmd, err := slack.SlashCommandParse(c.Request)
	if err != nil {
		ctrl.Error(c, fmt.Errorf("%w: parsing slash command: %w", stderror.ErrParsing, err))
		return
	}
	cmd.Text = strings.TrimSpace(cmd.Text)

	var resp *slack.Msg
	if cmd.Text == "" {
		resp, err = ctrl.lockService.Leaderboard(c.Request.Context())
	} else {
		resp, err = ctrl.lockService.Lock(c.Request.Context(), cmd)
	}
	if ctrl.Error(c, err) {
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (ctrl *Controller) reviewLoadHandler(c *gin.Context) {
	cmd, err := slack.SlashCommandParse(c.Request)
	if err != nil {
		ctrl.Error(c, fmt.Errorf("%w: parsing slash command: %w", stderror.ErrParsing, err))
		return
	}

	name := strings.TrimSpace(cmd.Text)
	if name == "" {
		repos, err := ctrl.reviewLoad.ReviewLoad(c.Request.Context())
		if ctrl.Error(c, err) {
			return
		}

		c.JSON(http.StatusOK, reviewLoadMessage("every repository", repos, true))
		return
	}

	repo, reviewers, err := ctrl.reviewLoad.RepoReviewLoad(c.Request.Context(), name)
	if unknownRepo, ok := errors.AsType[*reviewload.UnknownRepositoryError](err); ok {
		c.JSON(http.StatusOK, ephemeral(reviewLoadUsage(unknownRepo)))
		return
	}
	if ctrl.Error(c, err) {
		return
	}

	var repos []reviewload.RepositoryReviewers
	if len(reviewers) > 0 {
		repos = []reviewload.RepositoryReviewers{{Repo: repo, Reviewers: reviewers}}
	}
	c.JSON(http.StatusOK, reviewLoadMessage(repo.GetFullName(), repos, false))
}

// maxTableRows is Slack's limit on a table block's rows, header included.
const maxTableRows = 100

// reviewLoadMessage renders repos as a single table block, Slack allowing only one per message: one row
// per reviewer with links to their PRs. With repoRows, each repository's reviewers follow a row holding
// its name.
func reviewLoadMessage(scope string, repos []reviewload.RepositoryReviewers, repoRows bool) *slack.Msg {
	if len(repos) == 0 {
		return ephemeral(fmt.Sprintf("Nobody is reviewing an open PR of *%s*.", scope))
	}

	table := slack.NewTableBlock("").
		WithColumnSettings(
			slack.ColumnSetting{Align: slack.ColumnAlignmentLeft},
			slack.ColumnSetting{Align: slack.ColumnAlignmentRight},
			slack.ColumnSetting{Align: slack.ColumnAlignmentLeft, IsWrapped: true},
		).
		AddRow(boldCell("Reviewer"), boldCell("Score"), boldCell("PRs"))

	truncated := false
	addRow := func(cells ...slack.TableCell) {
		if len(table.Rows) == maxTableRows {
			truncated = true
			return
		}
		table.AddRow(cells...)
	}

	for _, repo := range repos {
		if repoRows {
			addRow(boldCell(repo.Repo.GetName()), slack.NewTableRawTextCell(""), slack.NewTableRawTextCell(""))
		}
		for _, r := range repo.Reviewers {
			links := make([]slack.RichTextSectionElement, 0, 2*len(r.PRs))
			for i, pr := range r.PRs {
				if i > 0 {
					links = append(links, slack.NewRichTextSectionTextElement(", ", nil))
				}
				links = append(links, slack.NewRichTextSectionLinkElement(pr.URL, fmt.Sprintf("#%d", pr.Number), nil))
			}

			addRow(
				slack.NewTableRawTextCell(r.Login),
				slack.NewTableRawTextCell(strconv.Itoa(r.Score)),
				slack.NewTableRichTextCell(slack.NewRichTextSection(links...)),
			)
		}
	}

	title := fmt.Sprintf("*Review load of %s*", scope)
	if truncated {
		title += fmt.Sprintf(" (first %d rows)", maxTableRows-1)
	}

	msg := ephemeral(title)
	msg.Blocks = slack.Blocks{BlockSet: []slack.Block{
		slack.NewSectionBlock(slack.NewTextBlockObject(slack.MarkdownType, title, false, false), nil, nil),
		table,
	}}

	return msg
}

func boldCell(text string) *slack.TableRichTextCell {
	return slack.NewTableRichTextCell(slack.NewRichTextSection(
		slack.NewRichTextSectionTextElement(text, &slack.RichTextSectionTextStyle{Bold: true})))
}

func reviewLoadUsage(err *reviewload.UnknownRepositoryError) string {
	names := make([]string, 0, len(err.Repositories))
	for _, name := range err.Repositories {
		names = append(names, "`"+name+"`")
	}

	return fmt.Sprintf("Unknown repository `%s`. Usage: `/review-load [repository]`, all repositories when omitted. Repositories: %s",
		err.Name, strings.Join(names, ", "))
}

func ephemeral(text string) *slack.Msg {
	return &slack.Msg{ResponseType: slack.ResponseTypeEphemeral, Text: text}
}
