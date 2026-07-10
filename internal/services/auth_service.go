package services

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

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
		return nil, fmt.Errorf("user not found: %w", err)
	}

	if user == nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err == nil {
		return user, nil
	}

	// Legacy rows store the password in plaintext; accept once and upgrade
	// the stored value to a bcrypt hash.
	if subtle.ConstantTimeCompare([]byte(user.Password), []byte(password)) == 1 {
		if hash, herr := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost); herr == nil {
			if uerr := s.queries.UpdateUserPassword(ctx, repository.UpdateUserPasswordParams{
				ID:       user.ID,
				Password: string(hash),
			}); uerr != nil {
				log.Printf("failed to upgrade password hash for user %d: %v", user.ID, uerr)
			}
		}
		return user, nil
	}

	return nil, fmt.Errorf("invalid credentials")
}
