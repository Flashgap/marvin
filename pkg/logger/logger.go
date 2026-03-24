package logger

import (
	"context"
	"os"
	"time"

	"github.com/Flashgap/logrus"
)

var (
	Standard = logrus.StandardLogger() // Alias.
)

type Config struct {
	GoogleCloudProject string
	// DefaultLogLevel is default log level used when verbose mode is disabled.
	// Verbose mode can be enabled by setting VerboseContextKey to true in context.
	DefaultLogLevel logrus.Level
	IsDevEnv        bool
}

// Init initializes loggers.
func Init(cfg Config) {
	// Init std logger:
	initLogger(Standard, cfg.IsDevEnv, cfg.GoogleCloudProject, cfg.DefaultLogLevel)
}

func initLogger(logger *logrus.Logger, isDevEnv bool, googleCloudProject string, level logrus.Level) {
	if isDevEnv { // Dev env.
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			PadLevelText:    true,
			TimestampFormat: time.TimeOnly,
			ForceColors:     true,
		})
	} else {
		logger.SetFormatter(&logrus.GCPFormatter{GoogleProjectID: googleCloudProject})
	}
	logger.SetOutput(os.Stdout)
	logger.SetLevel(level)
}

// WithContext creates an entry from the standard logger and adds a context to it.
func WithContext(ctx context.Context) *logrus.Entry {
	return logrus.WithContext(ctx)
}
