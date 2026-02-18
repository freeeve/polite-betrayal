package auth

import "context"

// SetUserIDForTest injects a user ID into the context for testing purposes.
func SetUserIDForTest(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}
