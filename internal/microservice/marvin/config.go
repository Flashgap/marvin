package marvin

import (
	"github.com/Flashgap/logrus"
	"github.com/kelseyhightower/envconfig"

	"github.com/Flashgap/marvin/internal/config"
	"github.com/Flashgap/marvin/internal/validate"
)

// Config inherits from Base and configures the marvin microservice.
type Config struct {
	config.Base
	config.Jira
	config.Github
	config.Slack
	config.Linear
	config.Marvin
}

func NewConfig() *Config {
	cfg := new(Config)
	if err := envconfig.Process("", cfg); err != nil {
		logrus.Fatal(err)
	}

	if err := validate.Struct(cfg); err != nil {
		logrus.Fatalf("invalid configuration: %v", err)
	}

	return cfg
}
