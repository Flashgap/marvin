package config

import (
	"github.com/Flashgap/marvin/pkg/cloud"
)

type CloudConfig = cloud.Config // A simple alias to embed a better name.

// Base is the base of all services and workers configuration.
type Base struct {
	CloudConfig `ignored:"true"`

	// EnableErrorReporting allows to enable/disable the GCP error reporting feature
	// it may be used to disable it in CI/local tests for instance
	EnableErrorReporting bool `envconfig:"ENABLE_ERROR_REPORTING" default:"true"`

	// Port for the http server
	Port string `envconfig:"PORT" default:"8080"`

	// LogLevel determines what type of log we print (info, error, etc.)
	// Defaults to 5 which is Info
	LogLevel int `envconfig:"LOG_LEVEL" default:"5"`

	// IsDevEnv checks whether we're in GAE or not to adapt the log format
	// Defaults to false
	IsDevEnv bool `envconfig:"IS_DEV_ENV"`
}

// GetLogLevel returns the log level.
func (cfg Base) GetLogLevel() int {
	return cfg.LogLevel
}

// GetGCloudProject returns the ProjectID.
func (cfg Base) GetGCloudProject() string {
	return cfg.ProjectID
}

// IsLocalEnv checks whether the current environment is local based on the ProjectID.
// Used by unittests to prevent dangerouns operations on remote resources.
func (cfg Base) IsLocalEnv() bool {
	return cfg.ProjectID == "demo-dev"
}

// GetIsDevEnv checks if we are in a development environment based on the IsDevEnv flag.
func (cfg Base) GetIsDevEnv() bool {
	return cfg.IsDevEnv
}
