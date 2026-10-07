package config

// Tasks configuration, shared by every /marvin/_task route.
type Tasks struct {
	// TasksSecret authenticates inbound task requests (e.g. from Cloud
	// Scheduler), sent as "Authorization: Bearer <secret>". When empty (and
	// not in dev), task requests are rejected.
	TasksSecret string `envconfig:"MARVIN_TASKS_SECRET" secret:"true"`
}
