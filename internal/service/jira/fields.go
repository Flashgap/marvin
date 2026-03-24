package jira

import (
	"fmt"
	"reflect"

	"github.com/Flashgap/marvin/internal/config"
)

type fields struct {
	ProjectKey              string
	ProjectID               string
	TaskIssueTypeID         string
	EpicIssueTypeID         string
	StartDateCustomFieldKey string
	InProgressTransitionID  string
	DoneTransitionID        string
}

func newFields(cfg *config.Jira) (*fields, error) {
	// This is only run at initialisation, so using reflect is not going to affect runtime performance much
	ret := fields{}
	reflectedConfig := reflect.ValueOf(&ret)
	nFields := reflectedConfig.Elem().NumField()

	if len(cfg.JiraFields) != nFields {
		return nil, fmt.Errorf("error parsing JiraFields, not all fields were provided. Expecting %d, got %d", nFields, len(cfg.JiraFields))
	}

	for k, v := range cfg.JiraFields {
		field := reflectedConfig.Elem().FieldByName(k)
		if !field.IsValid() {
			return nil, fmt.Errorf("unknown field %q in fields", k)
		}
		field.Set(reflect.ValueOf(v))
	}

	return &ret, nil
}
