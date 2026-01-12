package discovery

// ServiceRoute represents a dynamically discovered route.
type ServiceRoute struct {
	ServiceName string
	Prefix      string
	TargetURL   string
	Methods     []string
}
