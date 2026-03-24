package cloud

// Google Cloud Run provider.

import (
	"context"
	"fmt"
	"os"
)

const (
	cloudRunServiceEnvVar     = "K_SERVICE"  // Provided by Google Cloud Run to identify the service.
	cloudRunServiceVersionVar = "K_REVISION" // Provided by Google Cloud Run to identify the service version.
)

// load loads the Google metadata from the metadata server.
func (c *Config) loadGoogleCloudRun(ctx context.Context) error {
	if err := c.loadGoogleMetadata(ctx); err != nil {
		return fmt.Errorf("googleMetadataConfig.load: %w", err)
	}

	c.Service = os.Getenv(cloudRunServiceEnvVar)
	if c.Service == "" {
		return fmt.Errorf("missing %q environment variable", cloudRunServiceEnvVar)
	}

	c.ServiceVersion = os.Getenv(cloudRunServiceVersionVar)
	if c.ServiceVersion == "" {
		return fmt.Errorf("missing %q environment variable", cloudRunServiceVersionVar)
	}

	return nil
}
