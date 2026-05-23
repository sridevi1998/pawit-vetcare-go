package httpapi

import (
	"context"
	"net/http"
)

type requestIDContextKey struct{}

func withRequestIDContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDContextKey{}, id)
}

func requestID(r *http.Request) string {
	id, _ := r.Context().Value(requestIDContextKey{}).(string)
	return id
}
