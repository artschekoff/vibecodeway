package store

import "context"

// MessageWriter is a minimal interface for writing messages and resolving authors.
// Injected into provider services so they can persist messages without importing the full store.
type MessageWriter interface {
	ResolveAuthor(ctx context.Context, sourceID, externalName string) string
	UpsertMessage(ctx context.Context, args UpsertMessageArgs) (string, error)
}

// compile-time assertion
var _ MessageWriter = (*Store)(nil)
