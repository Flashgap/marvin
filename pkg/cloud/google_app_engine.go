package cloud

import (
	"context"
	"fmt"
	"os"
)

// load loads the Google metadata from the metadata server.
func (c *Config) loadGoogleAppEngine(ctx context.Context) error {
	if err := c.loadGoogleMetadata(ctx); err != nil {
		return fmt.Errorf("googleMetadataConfig.load: %w", err)
	}

	c.Service = os.Getenv("GAE_SERVICE")
	if c.Service == "" {
		return fmt.Errorf("missing GAE_SERVICE environment variable")
	}

	c.ServiceVersion = os.Getenv("GAE_VERSION")
	if c.ServiceVersion == "" {
		return fmt.Errorf("missing GAE_VERSION environment variable")
	}

	return nil
}
