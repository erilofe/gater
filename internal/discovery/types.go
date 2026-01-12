package discovery

// ServiceRoute represents a dynamically discovered route with load balancing support.
type ServiceRoute struct {
	ServiceName string
	Prefix      string
	TargetURLs  []string // Multiple target URLs for load balancing
	Methods     []string
}
