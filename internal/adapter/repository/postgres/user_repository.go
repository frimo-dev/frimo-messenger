package postgres

import (
	"context"
	"errors"
	"fmt"
	"uuid"

	"github.com/frimo-dev/frimo-messenger/internal/service/user"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool}
}

func (r *UserRepository) GetUserByID(ctx context.Context, userID uuid.UUID) (user.User, error) {
	const query = `
		SELECT email, username, name, surname, created_at
		FROM users
		WHERE id = $1;
	`

	var u user.User
	u.ID = userID

	err := r.pool.QueryRow(ctx, query, userID).Scan(&u.Email, &u.Username, &u.Name, &u.Surname, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return user.User{}, user.ErrUserNotFound
		}

		return user.User{}, fmt.Errorf("failed to get user by ID: %w", err)
	}

	return u, nil
}

func (r *UserRepository) GetPublicUserByUsername(ctx context.Context, username string) (user.PublicUser, error) {
	const query = `
		SELECT id, name, surname
		FROM users
		WHERE username = $1;
	`

	var publicUser user.PublicUser
	publicUser.Username = username

	err := r.pool.QueryRow(ctx, query, username).Scan(&publicUser.ID, &publicUser.Name, &publicUser.Surname)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return user.PublicUser{}, user.ErrUserNotFound
		}

		return user.PublicUser{}, fmt.Errorf("failed to get user by username: %w", err)
	}

	return publicUser, nil
}
