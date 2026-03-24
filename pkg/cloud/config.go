package cloud

import (
	"context"
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const environmentEnvVar = "ENVIRONMENT"

type Environment string

const (
	EnvironmentLocal      Environment = "local"
	EnvironmentAppEngine  Environment = "appengine"
	EnvironmentCloudRun   Environment = "cloudrun"
	EnvironmentKubernetes Environment = "kubernetes"
)

const (
	localFakeGCloudRegion = "dev-east1"
)

// Config is a generic cloud provider configuration.
type Config struct {
	Environment Environment

	// ProjectID is the ID of the project.
	ProjectID string

	// Region is the datacenter where the app is hosted (e.g. europe-west1)
	Region string

	// Zone is the zone in the DC where the app is hosted (e.g. europe-west1-c)
	Zone string

	// InstanceID gets the name or ID of the instance.
	InstanceID string

	// Service is the name of the service.
	Service string

	// ServiceVersion is the version of the service.
	ServiceVersion string

	// ServiceAccountEmail is the email associated with the service account.
	ServiceAccountEmail string
}

// Load loads the cloud provider metadata.
func (c *Config) Load(ctx context.Context) error {
	c.Environment = Environment(os.Getenv(environmentEnvVar))

	switch c.Environment {
	case EnvironmentAppEngine:
		return c.loadGoogleAppEngine(ctx)
	case EnvironmentCloudRun:
		return c.loadGoogleCloudRun(ctx)
	case EnvironmentLocal:
		c.ProjectID = os.Getenv("GOOGLE_CLOUD_PROJECT")
		c.InstanceID = uuid.NewString()
		c.Region = localFakeGCloudRegion
		c.Service = uuid.NewString()
	default:
		panic(fmt.Sprintf("Unknown environment: %s", c.Environment))
	}

	return nil
}

func (c *Config) MountProbes(router gin.IRoutes, readiness, liveness gin.HandlerFunc) {
	switch c.Environment {
	case EnvironmentAppEngine:
		router.GET("/_ah/warmup", readiness)
		router.GET("/health", liveness)
	case EnvironmentKubernetes, EnvironmentCloudRun:
		router.GET("/_probes/readiness", readiness)
		router.GET("/_probes/liveness", liveness)
	}
}
