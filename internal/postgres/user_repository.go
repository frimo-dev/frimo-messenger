package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/frimo-dev/frimo-messenger/internal/user"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Стандартный код ошибки PostgreSQL, означающий попытку нарушить уникальность ключа
const uniqueViolationCode = "23505"

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{
		pool: pool,
	}
}

func (ur *UserRepository) Create(ctx context.Context, email string, passwordHash string) (user.User, error) {
	createdUser := user.User{
		ID:        uuid.NewString(),
		Email:     email,
		CreatedAt: time.Now().UTC(),
	}

	const query = `INSERT INTO users (id, email, password_hash, created_at) VALUES ($1, $2, $3, $4)`

	_, err := ur.pool.Exec(
		ctx,
		query,
		createdUser.ID,
		createdUser.Email,
		passwordHash,
		createdUser.CreatedAt,
	)

	if err != nil {
		if isEmailUniqueViolation(err) {
			return user.User{}, user.ErrEmailAlreadyExists
		}

		return user.User{}, fmt.Errorf("insert user: %w", err)
	}
	return createdUser, nil
}

func isEmailUniqueViolation(err error) bool {
	var postgresError *pgconn.PgError

	if !errors.As(err, &postgresError) {
		return false
	}

	return postgresError.Code == uniqueViolationCode &&
		postgresError.ConstraintName == "users_email_unique_idx"
}

var _ user.Repository = (*UserRepository)(nil)
