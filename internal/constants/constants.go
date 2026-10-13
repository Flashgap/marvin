package constants

import "time"

const (
	// ShutdownTimeout graceful shutdown timeout.
	ShutdownTimeout = 3 * time.Second

	// ReadHeaderTimeout is used to mitigate possible Slowloris attacks.
	ReadHeaderTimeout = time.Second * 20

	// GoogleKeysTimeout bounds the fetch of Google's public keys, used to verify task tokens.
	GoogleKeysTimeout = 10 * time.Second
)
