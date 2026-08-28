package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v90/github"
)

// The types, accessors and MergeAsync below are copied from the go-github pull request adding
// asynchronous merge support (https://github.com/google/go-github/pull/4491), which is still open.
// Keeping the upstream names and shapes means that, once it is merged and released, this file can be
// dropped and the calls pointed at PullRequests.MergeAsync without touching the callers.
//
// Known limitation inherited from upstream: MergeAsync goes through client.Do, which discards the
// response body on a 4xx. GitHub reports a refused merge as a regular result payload on a 400
// (`{"status":"failed","details":{"message":"..."}}`), so on such a refusal the caller gets an
// *github.ErrorResponse with an empty Message instead of the reason the merge was refused.

// Statuses reported by GitHub's asynchronous merge endpoint.
const (
	MergeAsyncStatusPending  = "pending"
	MergeAsyncStatusMerged   = "merged"
	MergeAsyncStatusEnqueued = "enqueued"
	MergeAsyncStatusFailed   = "failed"
)

// MergeActionDirect merges the pull request right away instead of adding it to a merge queue.
const MergeActionDirect = "direct_merge"

// PullRequestMergeAsyncRequest represents a request to merge a pull request asynchronously.
type PullRequestMergeAsyncRequest struct {
	// MergeMethod is the merge method: merge, squash, or rebase. Not supported on merge_queue actions.
	MergeMethod *string `json:"merge_method,omitempty"`
	// MergeAction is how to merge: default, direct_merge, or merge_queue.
	MergeAction *string `json:"merge_action,omitempty"`
	// CommitTitle is the title for the automatic commit message. Not supported on merge_queue actions.
	CommitTitle *string `json:"commit_title,omitempty"`
	// CommitMessage is extra detail to append to the automatic commit message. Not supported on merge_queue actions.
	CommitMessage *string `json:"commit_message,omitempty"`
	// SHA that the pull request head must match to allow the merge.
	SHA *string `json:"sha,omitempty"`
}

// PullRequestMergeAsyncResult represents the current state of an asynchronous merge request.
type PullRequestMergeAsyncResult struct {
	Status  *string                       `json:"status,omitempty"`
	Details *PullRequestMergeAsyncDetails `json:"details,omitempty"`
}

// PullRequestMergeAsyncDetails represents details for the current state of a PullRequestMergeAsyncResult.
type PullRequestMergeAsyncDetails struct {
	Message         *string `json:"message,omitempty"`
	UUID            *string `json:"uuid,omitempty"`
	MergeMethod     *string `json:"merge_method,omitempty"`
	MergeAction     *string `json:"merge_action,omitempty"`
	ExpectedHeadSHA *string `json:"expected_head_sha,omitempty"`
	SHA             *string `json:"sha,omitempty"`
}

// GetExpectedHeadSHA returns the ExpectedHeadSHA field if it's non-nil, zero value otherwise.
func (p *PullRequestMergeAsyncDetails) GetExpectedHeadSHA() string {
	if p == nil || p.ExpectedHeadSHA == nil {
		return ""
	}
	return *p.ExpectedHeadSHA
}

// GetMergeAction returns the MergeAction field if it's non-nil, zero value otherwise.
func (p *PullRequestMergeAsyncDetails) GetMergeAction() string {
	if p == nil || p.MergeAction == nil {
		return ""
	}
	return *p.MergeAction
}

// GetMergeMethod returns the MergeMethod field if it's non-nil, zero value otherwise.
func (p *PullRequestMergeAsyncDetails) GetMergeMethod() string {
	if p == nil || p.MergeMethod == nil {
		return ""
	}
	return *p.MergeMethod
}

// GetMessage returns the Message field if it's non-nil, zero value otherwise.
func (p *PullRequestMergeAsyncDetails) GetMessage() string {
	if p == nil || p.Message == nil {
		return ""
	}
	return *p.Message
}

// GetSHA returns the SHA field if it's non-nil, zero value otherwise.
func (p *PullRequestMergeAsyncDetails) GetSHA() string {
	if p == nil || p.SHA == nil {
		return ""
	}
	return *p.SHA
}

// GetUUID returns the UUID field if it's non-nil, zero value otherwise.
func (p *PullRequestMergeAsyncDetails) GetUUID() string {
	if p == nil || p.UUID == nil {
		return ""
	}
	return *p.UUID
}

// GetCommitMessage returns the CommitMessage field if it's non-nil, zero value otherwise.
func (p *PullRequestMergeAsyncRequest) GetCommitMessage() string {
	if p == nil || p.CommitMessage == nil {
		return ""
	}
	return *p.CommitMessage
}

// GetCommitTitle returns the CommitTitle field if it's non-nil, zero value otherwise.
func (p *PullRequestMergeAsyncRequest) GetCommitTitle() string {
	if p == nil || p.CommitTitle == nil {
		return ""
	}
	return *p.CommitTitle
}

// GetMergeAction returns the MergeAction field if it's non-nil, zero value otherwise.
func (p *PullRequestMergeAsyncRequest) GetMergeAction() string {
	if p == nil || p.MergeAction == nil {
		return ""
	}
	return *p.MergeAction
}

// GetMergeMethod returns the MergeMethod field if it's non-nil, zero value otherwise.
func (p *PullRequestMergeAsyncRequest) GetMergeMethod() string {
	if p == nil || p.MergeMethod == nil {
		return ""
	}
	return *p.MergeMethod
}

// GetSHA returns the SHA field if it's non-nil, zero value otherwise.
func (p *PullRequestMergeAsyncRequest) GetSHA() string {
	if p == nil || p.SHA == nil {
		return ""
	}
	return *p.SHA
}

// GetDetails returns the Details field.
func (p *PullRequestMergeAsyncResult) GetDetails() *PullRequestMergeAsyncDetails {
	if p == nil {
		return nil
	}
	return p.Details
}

// GetStatus returns the Status field if it's non-nil, zero value otherwise.
func (p *PullRequestMergeAsyncResult) GetStatus() string {
	if p == nil || p.Status == nil {
		return ""
	}
	return *p.Status
}

// MergeAsync merges a pull request asynchronously. For stacked pull requests,
// this also merges everything below it in the stack. This is the required
// method for merging stacked pull requests; the legacy Merge method cannot be
// used for stacks.
//
// A pending response includes a UUID in PullRequestMergeAsyncResult.Details.UUID
// that must be passed to GetMergeAsyncResult to poll for the outcome.
//
// GitHub API docs: https://docs.github.com/rest/pulls/pulls?apiVersion=2022-11-28#merge-a-pull-request-asynchronously
//
//meta:operation PUT /repos/{owner}/{repo}/pulls/{pull_number}/merge-async
func (h *client) MergeAsync(ctx context.Context, owner, repo string, number int, body PullRequestMergeAsyncRequest) (*PullRequestMergeAsyncResult, *github.Response, error) {
	u := fmt.Sprintf("repos/%v/%v/pulls/%v/merge-async", owner, repo, number)

	req, err := h.NewRequest(ctx, "PUT", u, body)
	if err != nil {
		return nil, nil, err
	}

	var result *PullRequestMergeAsyncResult
	resp, err := h.Do(req, &result)
	if err != nil {
		return nil, resp, err
	}

	return result, resp, nil
}

// MergePRAsync merges the given PR of the repository the webhook comes from. It is the only merge
// endpoint accepting stacked PRs, the synchronous one rejects them with a 403.
func (h *client) MergePRAsync(ctx context.Context, webhook RepoSenderGetter, prNumber int, body PullRequestMergeAsyncRequest) (*PullRequestMergeAsyncResult, *github.Response, error) {
	return h.MergeAsync(ctx, webhook.GetRepo().GetOwner().GetLogin(), webhook.GetRepo().GetName(), prNumber, body)
}
