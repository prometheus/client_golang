package promhttp

import "context"

type _PathKey string

var requestPathKey _PathKey

func WithRequestPath(ctx context.Context, path string) context.Context {
	return context.WithValue(ctx, requestPathKey, path)
}

func pathFromContext(ctx context.Context) string {
	if value, ok := ctx.Value(requestPathKey).(string); ok {
		return value
	}
	return "*"
}
