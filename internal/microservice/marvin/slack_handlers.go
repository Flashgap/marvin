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
		reviewers, err := ctrl.reviewLoad.ReviewLoad(c.Request.Context())
		if ctrl.Error(c, err) {
			return
		}

		c.JSON(http.StatusOK, reviewLoadMessage("all repositories", reviewers, true))
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

	c.JSON(http.StatusOK, reviewLoadMessage(repo.GetFullName(), reviewers, false))
}

// maxTableRows is Slack's limit on a table block's rows, header included.
const maxTableRows = 100

// reviewLoadMessage renders reviewers as a table block, one row per reviewer with links to their PRs.
// withRepo prefixes each PR link with its repository, for rankings spanning several repositories.
func reviewLoadMessage(scope string, reviewers []reviewload.Reviewer, withRepo bool) *slack.Msg {
	if len(reviewers) == 0 {
		return ephemeral(fmt.Sprintf("Nobody is reviewing an open PR of *%s*.", scope))
	}

	title := fmt.Sprintf("*Review load of %s*", scope)
	if len(reviewers) > maxTableRows-1 {
		title += fmt.Sprintf(" (lowest %d of %d)", maxTableRows-1, len(reviewers))
		reviewers = reviewers[:maxTableRows-1]
	}

	table := slack.NewTableBlock("").
		WithColumnSettings(
			slack.ColumnSetting{Align: slack.ColumnAlignmentLeft},
			slack.ColumnSetting{Align: slack.ColumnAlignmentRight},
			slack.ColumnSetting{Align: slack.ColumnAlignmentLeft, IsWrapped: true},
		).
		AddRow(boldCell("Reviewer"), boldCell("Score"), boldCell("PRs"))
	for _, r := range reviewers {
		links := make([]slack.RichTextSectionElement, 0, 2*len(r.PRs))
		for i, pr := range r.PRs {
			if i > 0 {
				links = append(links, slack.NewRichTextSectionTextElement(", ", nil))
			}
			text := fmt.Sprintf("#%d", pr.Number)
			if withRepo {
				text = pr.Repo + text
			}
			links = append(links, slack.NewRichTextSectionLinkElement(pr.URL, text, nil))
		}

		table.AddRow(
			slack.NewTableRawTextCell(r.Login),
			slack.NewTableRawTextCell(strconv.Itoa(r.Score)),
			slack.NewTableRichTextCell(slack.NewRichTextSection(links...)),
		)
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
