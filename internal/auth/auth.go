package auth

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type contextKey struct{}

// ParseUserTokens parses "alice:tok_abc;bob:tok_def" into a map of token→username.
func ParseUserTokens(envStr string) map[string]string {
	m := make(map[string]string)
	if envStr == "" {
		return m
	}
	for _, pair := range strings.Split(envStr, ";") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			log.Printf("WARN: skipping invalid user token entry: %q", pair)
			continue
		}
		m[parts[1]] = parts[0]
	}
	return m
}

// Middleware returns HTTP middleware that authenticates requests via Bearer token
// or ?token= query parameter. If tokens is empty, single-user mode is used.
func Middleware(tokens map[string]string) func(http.Handler) http.Handler {
	singleUser := len(tokens) == 0

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if singleUser {
				ctx := context.WithValue(r.Context(), contextKey{}, "default")
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			token := extractToken(r)
			if token == "" {
				http.Error(w, `{"error":"missing token"}`, http.StatusUnauthorized)
				return
			}

			username, ok := tokens[token]
			if !ok {
				http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), contextKey{}, username)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractToken(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); auth != "" {
		if strings.HasPrefix(auth, "Bearer ") {
			return strings.TrimPrefix(auth, "Bearer ")
		}
	}
	return r.URL.Query().Get("token")
}

// GetUser returns the authenticated username from the request context.
func GetUser(ctx context.Context) string {
	if v, ok := ctx.Value(contextKey{}).(string); ok {
		return v
	}
	return ""
}

// EnsureUserDir creates the user directory structure under dataDir if it doesn't exist.
// Returns the user's base directory path.
func EnsureUserDir(dataDir, username string) (string, error) {
	base := UserDir(dataDir, username)
	for _, sub := range []string{"repo", "data"} {
		dir := filepath.Join(base, sub)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
	}
	return base, nil
}

// UserDir returns the base directory for a user.
func UserDir(dataDir, username string) string {
	return filepath.Join(dataDir, "users", username)
}
