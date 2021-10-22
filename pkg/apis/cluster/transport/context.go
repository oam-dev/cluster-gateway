package multicluster

import "context"

type contextKey string

const (
	// ClusterContextKey is the name of cluster using in client http context
	clusterContextKey = contextKey("ClusterName")
)

func WithMultiClusterContext(ctx context.Context, clusterName string) context.Context {
	return context.WithValue(ctx, clusterContextKey, clusterName)
}

func GetMultiClusterContext(ctx context.Context) (string, bool) {
	clusterName, ok := ctx.Value(clusterContextKey).(string)
	return clusterName, ok
}
