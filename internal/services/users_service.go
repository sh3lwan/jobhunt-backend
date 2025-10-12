package services

import (
	"context"

	"github.com/sh3lwan/jobhunter/internal/models"
	"github.com/sh3lwan/jobhunter/internal/repository"
)

type UserService struct {
	queries *repository.Queries
}

func NewUserService(queries *repository.Queries) *UserService {
	return &UserService{
		queries: queries,
	}
}

func (s *UserService) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {

	user, err := s.queries.GetUserByUsername(ctx, username)

	if err != nil {
		return nil, err
	}

	return &models.User{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		Password: user.Password,
	}, nil
}
