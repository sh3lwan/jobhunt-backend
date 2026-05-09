package utils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/golang-jwt/jwt/v5"
	"github.com/sh3lwan/jobhunter/internal/models"
)

func RespondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

func RespondError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "error",
		"message": message,
	})
}

// Helper function to get environment variable or default value
func GetEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func GetUserFromContext(c context.Context) (*models.User, error) {
	claims, ok := c.Value("claims").(jwt.MapClaims)

	if !ok {
		return nil, errors.New("no claims found in context")
	}

	userId, ok := claims["sub"].(float64)

	if !ok {
		return nil, fmt.Errorf("sub claim not found")
	}

	username, err := claims.GetIssuer()
	if err != nil {
		return nil, err
	}

	return &models.User{
		ID:       int64(userId),
		Username: username,
	}, nil
}
