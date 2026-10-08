package marvin

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	gogithub "github.com/google/go-github/v90/github"
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

	repo, reviewers, err := ctrl.reviewLoad.RepoReviewLoad(c.Request.Context(), strings.TrimSpace(cmd.Text))
	if unknownRepo, ok := errors.AsType[*reviewload.UnknownRepositoryError](err); ok {
		c.JSON(http.StatusOK, ephemeral(reviewLoadUsage(unknownRepo)))
		return
	}
	if ctrl.Error(c, err) {
		return
	}

	c.JSON(http.StatusOK, reviewLoadMessage(repo, reviewers))
}

// maxTableRows is Slack's limit on a table block's rows, header included.
const maxTableRows = 100

// reviewLoadMessage renders reviewers as a table block, one row per reviewer with links to their PRs.
func reviewLoadMessage(repo *gogithub.Repository, reviewers []reviewload.Reviewer) *slack.Msg {
	if len(reviewers) == 0 {
		return ephemeral(fmt.Sprintf("Nobody is reviewing an open PR of *%s*.", repo.GetFullName()))
	}

	title := fmt.Sprintf("*Review load of %s*", repo.GetFullName())
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
		for i, number := range r.PRs {
			if i > 0 {
				links = append(links, slack.NewRichTextSectionTextElement(", ", nil))
			}
			links = append(links, slack.NewRichTextSectionLinkElement(
				fmt.Sprintf("%s/pull/%d", repo.GetHTMLURL(), number), fmt.Sprintf("#%d", number), nil))
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

	prefix := "Usage: `/review-load <repository>`."
	if err.Name != "" {
		prefix = fmt.Sprintf("Unknown repository `%s`.", err.Name)
	}

	return fmt.Sprintf("%s Repositories: %s", prefix, strings.Join(names, ", "))
}

func ephemeral(text string) *slack.Msg {
	return &slack.Msg{ResponseType: slack.ResponseTypeEphemeral, Text: text}
}
