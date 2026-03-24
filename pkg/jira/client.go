//go:generate mockgen --source=$GOFILE --destination=mock/mock.go --package mock_jira
package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	restAPI                = "/rest/api/latest"
	issueResource          = "/issue"
	issueTransitionService = "/transitions"
	issueWorklogService    = "/worklog"
	searchResource         = "/search"
)

type Client interface {
	// SearchIssue searches for issues where the summary substring fuzzy matches the given string in the given project
	SearchIssue(ctx context.Context, projectName string, summarySearchString string) (*SearchResponse, error)
	// CreateIssue creates an issue in Jira with the given payload
	CreateIssue(ctx context.Context, payload map[string]any) (string, error)
	// TransitionIssue transitions the given issue to the transition ID
	TransitionIssue(ctx context.Context, issueID, transitionID string) error
	// AddIssueWorklog adds a worklog to the given issue
	AddIssueWorklog(ctx context.Context, issueID string, hoursSpent float64) error
}

type client struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

// NewClient returns an initialized http client with a pre-configured timeout. Use this client to perform Jira requests
func NewClient(host, token string) (Client, error) {
	return &client{
		httpClient: &http.Client{Timeout: 10 * time.Minute},
		baseURL:    host,
		token:      token,
	}, nil
}

func (c *client) SearchIssue(ctx context.Context, projectName, summarySearchString string) (*SearchResponse, error) {
	params := make(url.Values)
	params.Add("jql", fmt.Sprintf("project = %s AND summary ~ %q", projectName, summarySearchString))

	resp, err := c.do(ctx, http.MethodGet, params, http.NoBody, searchResource)
	if err != nil {
		return nil, fmt.Errorf("error performing request: %w", err)
	}
	defer resp.Body.Close()

	var searchResponse SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResponse); err != nil {
		return nil, fmt.Errorf("error decoding response JSON: %w", err)
	}

	return &searchResponse, nil
}

func (c *client) CreateIssue(ctx context.Context, payload map[string]any) (string, error) {
	payloadBytes := new(bytes.Buffer)
	if err := json.NewEncoder(payloadBytes).Encode(payload); err != nil {
		return "", fmt.Errorf("error encoding payload: %w", err)
	}

	resp, err := c.do(ctx, http.MethodPost, nil, payloadBytes, issueResource)
	if err != nil {
		return "", fmt.Errorf("error performing request: %w", err)
	}
	defer resp.Body.Close()

	var issue Issue
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return "", fmt.Errorf("error decoding response JSON: %w", err)
	}

	return issue.ID, nil
}

func (c *client) TransitionIssue(ctx context.Context, issueID, transitionID string) error {
	transitionRequest := TransitionRequest{
		Transition: Transition{
			ID: transitionID,
		},
	}

	payload := new(bytes.Buffer)
	if err := json.NewEncoder(payload).Encode(transitionRequest); err != nil {
		return fmt.Errorf("error encoding payload: %w", err)
	}

	resp, err := c.do(ctx, http.MethodPost, nil, payload, issueResource, issueID, issueTransitionService)
	if err != nil {
		return fmt.Errorf("error performing request: %w", err)
	}

	return resp.Body.Close()
}

func (c *client) AddIssueWorklog(ctx context.Context, issueID string, hoursSpent float64) error {
	t, err := time.ParseDuration(fmt.Sprintf("%.2fh", hoursSpent))
	if err != nil {
		return fmt.Errorf("error parsing duration: %w", err)
	}

	worklogRequest := WorklogRequest{
		TimeSpentSeconds: int64(t.Seconds()),
	}

	payload := new(bytes.Buffer)
	if err := json.NewEncoder(payload).Encode(worklogRequest); err != nil {
		return fmt.Errorf("error encoding payload: %w", err)
	}

	resp, err := c.do(ctx, http.MethodPost, nil, payload, issueResource, issueID, issueWorklogService)
	if err != nil {
		return fmt.Errorf("error performing request: %w", err)
	}

	return resp.Body.Close()
}

func (c *client) do(ctx context.Context, method string, params url.Values, body io.Reader, pathElems ...string) (*http.Response, error) {
	elems := append([]string{restAPI}, pathElems...)
	reqURL, err := url.JoinPath(c.baseURL, elems...)
	if err != nil {
		return nil, fmt.Errorf("error building request URL: %w", err)
	}

	if params != nil {
		reqURL += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Add("Authorization", "Basic "+c.token)
	req.Header.Add("Accept", "application/json")
	if method == http.MethodPost {
		req.Header.Add("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error performing request: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		defer resp.Body.Close()
		bod, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("error status in response: %s %s", resp.Status, string(bod))
	}

	return resp, nil
}
