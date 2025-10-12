package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/sh3lwan/jobhunter/internal/services"
	"github.com/sh3lwan/jobhunter/pkg/utils"
)

type AuthMiddleware struct {
	authService *services.AuthService
}

func NewAuthMiddleware(authService *services.AuthService) *AuthMiddleware {
	return &AuthMiddleware{
		authService: authService,
	}

}

func (am *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenstr := r.Header.Get("Authorization")

		tokenstr = strings.TrimPrefix(tokenstr, "Bearer ")

		if tokenstr == "" {
			utils.RespondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Missing or malformed token"})
			return
		}

		token, err := am.authService.Validate(tokenstr)
		if err != nil || !token.Valid {
			utils.RespondJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
			return
		}
		ctx := context.WithValue(r.Context(), "claims", token.Claims)

		r = r.WithContext(ctx)

		// Token is valid, proceed to the next handler
		next.ServeHTTP(w, r)
	})
}
