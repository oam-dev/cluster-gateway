package options

var (
	// OCMIntegration indicates whether to load cluster information from
	// the hosting cluster via OCM's cluster api. After enabling this option,
	// no caBundle and apiserver's URL are required in the cluster secret.
	// NOTE: This option only works in "non-etcd" mode.
	OCMIntegration = false
)
