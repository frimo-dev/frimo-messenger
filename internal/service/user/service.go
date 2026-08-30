package user

import (
	"context"
	"fmt"
	"time"
	"uuid"
)

type User struct {
	ID        uuid.UUID
	Email     string
	CreatedAt time.Time
}

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) GetUserByID(ctx context.Context, userID uuid.UUID) (User, error) {
	user, err := s.repository.GetUserByID(ctx, userID)
	if err != nil {
		return User{}, fmt.Errorf("failed to get user by ID: %w", err)
	}
	return user, nil
}
