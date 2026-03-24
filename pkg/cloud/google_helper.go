package cloud

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"cloud.google.com/go/compute/metadata"
)

const (
	gcloudLocationEU = "eu"
	gcloudLocationUS = "us"
)

// loadGoogleMetadata loads the Google metadata from the metadata server.
func (c *Config) loadGoogleMetadata(ctx context.Context) error {
	metadataClient := metadata.NewClient(http.DefaultClient)

	var err error
	c.ProjectID, err = metadataClient.ProjectIDWithContext(ctx)
	if err != nil {
		return fmt.Errorf("metadata.ProjectID: %w", err)
	}

	c.Zone, err = metadataClient.ZoneWithContext(ctx)
	if err != nil {
		return fmt.Errorf("metadata.Zone: %w", err)
	}

	c.InstanceID, err = metadataClient.InstanceNameWithContext(ctx)
	if err != nil {
		c.InstanceID, err = metadataClient.InstanceIDWithContext(ctx)
		if err != nil {
			return fmt.Errorf("metadata.InstanceName: %w", err)
		}
	}

	if a := strings.Split(c.Zone, "-"); len(a) > 1 {
		c.Region = strings.Join(strings.Split(c.Zone, "-")[0:2], "-")
	} else {
		c.Region = c.Zone
		// This is a hack around the metadata being weird on GAE
		if strings.HasPrefix(c.Zone, gcloudLocationEU) {
			c.Region = "europe-west1"
		} else if strings.HasPrefix(c.Zone, gcloudLocationUS) {
			c.Region = "us-central1"
		}
	}

	c.ServiceAccountEmail, err = metadataClient.EmailWithContext(ctx, "default")
	if err != nil {
		return fmt.Errorf("metadata.Email: %w", err)
	}

	return nil
}
