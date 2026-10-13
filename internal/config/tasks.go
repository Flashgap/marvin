package config

// Tasks configuration, shared by every /marvin/_task route.
//
// Task requests (e.g. from Cloud Scheduler) authenticate with a Google-signed
// OIDC token, sent as "Authorization: Bearer <token>". Outside dev, when either
// field is empty, task requests are rejected.
type Tasks struct {
	// TasksAudience is the audience the tokens are minted for: the job's
	// --oidc-token-audience, e.g. "https://marvin.example.com".
	TasksAudience string `envconfig:"MARVIN_TASKS_AUDIENCE"`
	// TasksServiceAccount is the email of the service account the tokens are
	// minted for: the job's --oidc-service-account-email.
	TasksServiceAccount string `envconfig:"MARVIN_TASKS_SERVICE_ACCOUNT"`
}

// TasksConfigured reports whether task authentication is set up.
func (cfg Tasks) TasksConfigured() bool {
	return cfg.TasksAudience != "" && cfg.TasksServiceAccount != ""
}
