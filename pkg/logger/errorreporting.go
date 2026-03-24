package logger

import (
	"context"
	"fmt"

	"cloud.google.com/go/errorreporting"
	"github.com/Flashgap/logrus"
)

type ErrorReportingConfig struct {
	ProjectID   string
	ServiceName string
}

func NewErrorReportingService(ctx context.Context, config ErrorReportingConfig) (*errorreporting.Client, error) {
	client, err := errorreporting.NewClient(ctx, config.ProjectID, errorreporting.Config{
		ServiceName: config.ServiceName,
		OnError: func(err error) {
			logrus.Errorf("error reporting error to Google: %v", err)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("error initializing Google Cloud Error Reporting client: %w", err)
	}

	return client, nil
}
