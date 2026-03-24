package jira

const (
	JSONKeyDescription      = "description"
	JSONKeyDueDate          = "duedate"
	JSONKeyID               = "id"
	JSONKeyIssueType        = "issuetype"
	JSONKeyLabels           = "labels"
	JSONKeyOriginalEstimate = "originalEstimate"
	JSONKeyParent           = "parent"
	JSONKeyProject          = "project"
	JSONKeySummary          = "summary"
	JSONKeyTimeTracking     = "timetracking"
	JSONKeyWebLink          = "webLink"
)

type SearchResponse struct {
	Total  int     `json:"total"`
	Issues []Issue `json:"issues"`
}

type Issue struct {
	ID string `json:"id"`
}

type WorklogResponse struct {
	Total    int       `json:"total"`
	Worklogs []Worklog `json:"worklogs"`
}

type Worklog struct {
	ID string `json:"id"`
}

type TransitionRequest struct {
	Transition Transition `json:"transition"`
}

type Transition struct {
	ID string `json:"id"`
}

type WorklogRequest struct {
	TimeSpentSeconds int64 `json:"timeSpentSeconds"`
}

type IssueRequest struct {
	Fields map[string]any `json:"fields"`
}
