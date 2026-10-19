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
	Username  string
	Name      string
	Surname   string
	CreatedAt time.Time
}

type PublicUser struct {
	ID       uuid.UUID
	Username string
	Name     string
	Surname  *string
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

func (s *Service) GetPublicUserByUsername(ctx context.Context, username string) (PublicUser, error) {
	user, err := s.repository.GetPublicUserByUsername(ctx, username)
	if err != nil {
		return PublicUser{}, fmt.Errorf("get public user by username: %w", err)
	}

	return user, nil
}
