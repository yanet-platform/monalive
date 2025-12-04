package requestid

import (
	"context"

	"github.com/google/uuid"
)

// RequestID is a unique identifier for a request.
type RequestID string

const HeaderKey = "X-Request-ID"

// key is an unexported type for context keys defined in this package.
// This prevents collisions with keys defined in other packages.
type contextKey int

// requestIDContextKey is the key for [RequestID] in Contexts. It is unexported;
// clients use requestid.NewContext and requestid.FromContext instead of using
// this key directly.
var requestIDContextKey contextKey

// Generate generates a new request ID.
func Generate() RequestID {
	return RequestID(uuid.New().String())
}

// FromContext returns the RequestID value stored in ctx, if any.
func FromContext(ctx context.Context) (RequestID, bool) {
	id, exists := ctx.Value(requestIDContextKey).(RequestID)
	return id, exists
}

// NewContext returns a new Context that carries value requestID.
func NewContext(ctx context.Context, requestID RequestID) context.Context {
	return context.WithValue(ctx, requestIDContextKey, requestID)
}
