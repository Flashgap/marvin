package jira

type Issue struct {
	ID                 string
	Title              string
	Description        string
	Estimate           float64
	CompletedAt        string
	ProjectName        string
	ProjectID          string
	ProjectDescription string
	ProjectStartDate   string
	ProjectTargetDate  string
	ProjectCompletedAt string
	URL                string
}
