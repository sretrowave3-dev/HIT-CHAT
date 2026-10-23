//go:build !devauth

package transport

// registerDevAuthRoutes is a no-op in standard/release builds.
// The /api/v1/dev/login endpoint does not exist and returns 404.
func (r *Router) registerDevAuthRoutes() {
	// Dev auth is entirely omitted from binary
}
