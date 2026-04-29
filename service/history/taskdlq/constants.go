package taskdlq

var (
	// DefaultClusterAttributeScope is used to write tasks to the DLQ for the domains default ActiveCluster.
	DefaultClusterAttributeScope = "cluster-attribute-default-scope"
	// DefaultClusterAttributeName is used to write tasks to the DLQ for the domains default ActiveCluster.
	DefaultClusterAttributeName = "cluster-attribute-default-name"
)
