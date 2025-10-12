package services

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/sh3lwan/jobhunter/internal/models"
	"github.com/sh3lwan/jobhunter/internal/repository"
)

type AuthService struct {
	jwtSecret string
	queries   *repository.Queries
}

func NewAuthService(jwtSecret string, queries *repository.Queries) *AuthService {
	return &AuthService{
		jwtSecret: jwtSecret,
		queries:   queries,
	}
}

func (s *AuthService) Generate(u *models.User) (string, error) {
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": u.ID,
		"iss": u.Username,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Hour * 72).Unix(),
	})

	return t.SignedString([]byte(s.jwtSecret))
}

func (s *AuthService) Validate(tokenStr string) (*jwt.Token, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrTokenMalformed
		}
		return []byte(s.jwtSecret), nil
	})

	return token, err
}

func (s *AuthService) ValidateUser(ctx context.Context, username, password string) (*models.User, error) {
	svc := NewUserService(s.queries)

	user, err := svc.GetUserByUsername(ctx, username)

	if err != nil {
		return nil, fmt.Errorf("user not found: %v", err)
	}

	if user != nil && user.Password == password {
		return user, nil
	}

	return nil, fmt.Errorf("invalid credentials")
}
