//go:generate mockgen --source=$GOFILE --destination=mock/mock.go --package mock_jira
package jira

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Flashgap/marvin/internal/config"
	"github.com/Flashgap/marvin/internal/middlewares"
	"github.com/Flashgap/marvin/pkg/jira"
)

const (
	labelKFP              = "KeyFinanceProject"
	labelYourOrganization = "YourOrganization"
)

var ErrNotFound = errors.New("jira: not found")

type Service interface {
	// DoCapReportWorkflow performs the cap report flow. Creating the necessary epics and tasks onto Jira, given a Linear issue
	DoCapReportWorkflow(ctx context.Context, issue *Issue, timeSpent float64) error
}

type service struct {
	jira.Client
	fields *fields
}

// NewService returns a new JIRA service
func NewService(cfg *config.Jira, client jira.Client) (Service, error) {
	f, err := newFields(cfg)
	if err != nil {
		return nil, fmt.Errorf("error parsing jira fields from fields: %w", err)
	}

	return &service{
		Client: client,
		fields: f,
	}, nil
}

func (s *service) DoCapReportWorkflow(ctx context.Context, issue *Issue, timeSpent float64) error {
	log := middlewares.LoggerFromGHContext(ctx, "jira.DoCapReportWorkflow")

	// Look to see if issue exists
	taskID, err := s.searchIssue(ctx, issue.ID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("error searching for task %q: %w", issue.ID, err)
	}

	// If it does, update time spent and exit
	if err == nil {
		log.Infof("Found existing issue %q, incrementing logged time by %.2f hours", fmt.Sprintf("[%s] %s", issue.ID, issue.Title), timeSpent)
		return s.AddIssueWorklog(ctx, taskID, timeSpent)
	}

	// If it doesn't, also look to see if the epic (which represents the project) exists
	epicID, err := s.searchIssue(ctx, issue.ProjectID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("error searching for epic %q: %w", issue.ProjectID, err)
	}

	// If it doesn't, create it
	if errors.Is(err, ErrNotFound) {
		log.Infof("Epic doesn't exist yet. Creating epic %q", fmt.Sprintf("[%s] %s", issue.ProjectID, issue.ProjectName))
		if epicID, err = s.CreateIssue(ctx, s.newEpic(issue)); err != nil {
			return fmt.Errorf("error creating epic: %w", err)
		}
		transitionID := s.fields.InProgressTransitionID
		if issue.ProjectCompletedAt != "" {
			log.Infof("Transitioning epic to the \"Done\" column")
			transitionID = s.fields.DoneTransitionID
		} else {
			log.Infof("Transitioning epic to the \"In Progress\" column")
		}
		// Move it to either the InProgress or Done column depending on status
		if err := s.TransitionIssue(ctx, epicID, transitionID); err != nil {
			return fmt.Errorf("error transitioning epic: %w", err)
		}
	}

	// Create the task
	log.Infof("Creating task %q", fmt.Sprintf("[%s] %s", issue.ID, issue.Title))
	taskID, err = s.CreateIssue(ctx, s.newTask(issue, epicID))
	if err != nil {
		return fmt.Errorf("error creating task: %w", err)
	}

	// Move the task to the done column
	log.Infof("Transitioning task to the \"Done\" column")
	if err := s.TransitionIssue(ctx, taskID, s.fields.DoneTransitionID); err != nil {
		return fmt.Errorf("error transitioning task: %w", err)
	}

	// Log time spent on the task
	log.Infof("Incrementing logged time by %.2f hours", timeSpent)
	return s.AddIssueWorklog(ctx, taskID, timeSpent)
}

// searchIssue searches through Jira issues summary for a substring fuzzy match of term
// we typically use either the Linear task ID (e.g. ENG-XXX) for tasks, and Linear project SlugID for Epics.
// With the current system this is only ever expected to find no more than 1 issue. It errors otherwise.
func (s *service) searchIssue(ctx context.Context, term string) (string, error) {
	searchResponse, err := s.SearchIssue(ctx, s.fields.ProjectKey, term)
	if err != nil {
		return "", fmt.Errorf("error searching for issue: %w", err)
	}

	if searchResponse.Total > 1 {
		return "", fmt.Errorf("ambiguous search string returned %d results", searchResponse.Total)
	}

	if searchResponse.Total == 0 {
		return "", ErrNotFound
	}

	return searchResponse.Issues[0].ID, nil
}

func (s *service) newEpic(issue *Issue) map[string]any {
	epic := map[string]any{
		jira.JSONKeyDescription: issue.ProjectDescription,
		jira.JSONKeyIssueType:   map[string]string{jira.JSONKeyID: s.fields.EpicIssueTypeID},
		jira.JSONKeyLabels:      []string{labelKFP, labelYourOrganization, quarterLabel(issue.CompletedAt)},
		jira.JSONKeyProject:     map[string]string{jira.JSONKeyID: s.fields.ProjectID},
		jira.JSONKeySummary:     fmt.Sprintf("[%s] %s", issue.ProjectID, issue.ProjectName),
		jira.JSONKeyWebLink:     issue.URL,
	}

	dueDate, err := time.Parse(time.DateOnly, issue.ProjectTargetDate)
	if err == nil {
		epic[jira.JSONKeyDueDate] = dueDate
	}

	startDate, err := time.Parse(time.DateOnly, issue.ProjectStartDate)
	if err == nil {
		epic[s.fields.StartDateCustomFieldKey] = startDate
	}

	return map[string]any{"fields": epic}
}

func (s *service) newTask(issue *Issue, epicID string) map[string]any {
	task := map[string]any{
		jira.JSONKeyDescription:  issue.Description,
		jira.JSONKeyIssueType:    map[string]string{jira.JSONKeyID: s.fields.TaskIssueTypeID},
		jira.JSONKeyParent:       map[string]string{jira.JSONKeyID: epicID},
		jira.JSONKeyProject:      map[string]string{jira.JSONKeyID: s.fields.ProjectID},
		jira.JSONKeySummary:      fmt.Sprintf("[%s] %s", issue.ID, issue.Title),
		jira.JSONKeyTimeTracking: map[string]string{jira.JSONKeyOriginalEstimate: fmt.Sprintf("%.2fh", issue.Estimate)},
	}

	return map[string]any{"fields": task}
}

// quarterLabel takes a RFC339 formatted date, and converts it into a string of the form Qx-YYYY where Q stands for quarter
// For example:
// 2019-10-12T07:20:50.52Z -> Q4-2019
// 2023-02-12T04:12:32.21Z -> Q1-2023
//
// If the rfc3339Date parameter is empty or poorly formatted, time.Now() is taken instead to compute the label
func quarterLabel(rfc3339Date string) string {
	t := time.Now().UTC()
	if rfc3339Date != "" {
		tStar, err := time.Parse(time.RFC3339, rfc3339Date)
		if err == nil {
			t = tStar
		}
	}

	return fmt.Sprintf("Q%d-%d", (t.Month()+2)/3, t.Year())
}
