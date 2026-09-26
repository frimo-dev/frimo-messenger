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
		SELECT email, created_at
		FROM users
		WHERE id = $1;
	`

	var u user.User
	u.ID = userID

	err := r.pool.QueryRow(ctx, query, userID).Scan(&u.Email, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return user.User{}, user.ErrUserNotFound
		}

		return user.User{}, fmt.Errorf("failed to get user by ID: %w", err)
	}

	return u, nil
}
