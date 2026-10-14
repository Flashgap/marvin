package constants

import "time"

const (
	// ShutdownTimeout graceful shutdown timeout.
	ShutdownTimeout = 3 * time.Second

	// ReadHeaderTimeout is used to mitigate possible Slowloris attacks.
	ReadHeaderTimeout = time.Second * 20
)
