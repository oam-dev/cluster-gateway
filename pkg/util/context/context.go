package context

import "context"

type contextKeyClusterName string

var (
	key contextKeyClusterName = ""
)

func WithClusterName(ctx context.Context, clusterName string) context.Context {
	return context.WithValue(ctx, key, clusterName)
}

func GetClusterName(ctx context.Context) string {
	return ctx.Value(key).(string)
}
