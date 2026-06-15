package middleware

import (
	"context"
	"net/http"
	"rosdomofon-portal/internal/auth"
)

type contextKey string

const (
	OwnerIDKey contextKey = "owner_id"
	FlatIDsKey contextKey = "flat_ids"
)

func Auth(jwtManager *auth.JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := r.Header.Get("Authorization")
			if token == "" {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			if len(token) > 7 && token[:7] == "Bearer " {
				token = token[7:]
			}

			claims, err := jwtManager.Verify(token)
			if err != nil {
				http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), OwnerIDKey, claims.OwnerID)
			ctx = context.WithValue(ctx, FlatIDsKey, claims.FlatIDs)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
