package config

import (
	"github.com/Flashgap/logrus"

	"github.com/Flashgap/marvin/pkg/logger"
)

// LoggerConfig returns a package configuration from the base configuration.
func (cfg Base) LoggerConfig() logger.Config {
	return logger.Config{
		GoogleCloudProject: cfg.ProjectID,
		DefaultLogLevel:    logrus.Level(cfg.LogLevel),
		IsDevEnv:           cfg.IsDevEnv,
	}
}

// ErrorReportingConfig returns a package configuration from the base configuration.
func (cfg Base) ErrorReportingConfig() logger.ErrorReportingConfig {
	return logger.ErrorReportingConfig{
		ProjectID:   cfg.ProjectID,
		ServiceName: cfg.Service,
	}
}
