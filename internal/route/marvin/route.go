package marvin

import (
	"path"

	"github.com/Flashgap/marvin/internal/route"
)

// BasePath is the usvc routing path.
const BasePath = "marvin"

// Paths is the unique module http routes.
var Paths = route.RoutingPaths{
	Endpoints: "/" + path.Join(BasePath, route.RoutingPathEndpointsPrefix),
	Tasks:     "/" + path.Join(BasePath, route.RoutingPathTasksPrefix),
	WebHooks:  "/" + path.Join(BasePath, route.RoutingPathWebHooksPrefix),
}

// Routes names.
const (
	Github        = "github"  // GitHub controller route
	GithubWebhook = "webhook" // Sub path of Github

	Linear         = "linear"   // Linear controller route
	LinearLogin    = "login"    // Sub path of Linear
	LinearCallback = "callback" // Sub path of Linear
)
