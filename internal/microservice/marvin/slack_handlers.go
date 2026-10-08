package marvin

import (
	"errors"
	"fmt"
	"net/http"
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

	c.JSON(http.StatusOK, ephemeral(reviewLoadText(repo, reviewers)))
}

func reviewLoadText(repo *gogithub.Repository, reviewers []reviewload.Reviewer) string {
	if len(reviewers) == 0 {
		return fmt.Sprintf("Nobody is reviewing an open PR of *%s*.", repo.GetFullName())
	}

	var b strings.Builder
	fmt.Fprintf(&b, "*Review load of %s*\n", repo.GetFullName())
	for _, r := range reviewers {
		links := make([]string, 0, len(r.PRs))
		for _, number := range r.PRs {
			links = append(links, fmt.Sprintf("<%s/pull/%d|#%d>", repo.GetHTMLURL(), number, number))
		}
		fmt.Fprintf(&b, "• *%s* — %d — %s\n", r.Login, r.Score, strings.Join(links, ", "))
	}

	return b.String()
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
