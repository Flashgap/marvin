package route

const (
	// V1Path is the relative path to reach this api version
	V1Path = "v1.0"
)

// Routing Paths prefixes.
const (
	RoutingPathEndpointsPrefix = "_ep"
	RoutingPathTasksPrefix     = "_task"
	RoutingPathWebHooksPrefix  = "_webhook"
)

// RoutingPaths wraps modules various paths.
type RoutingPaths struct {
	Endpoints string
	Tasks     string
	WebHooks  string
}
