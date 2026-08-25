package auth

import (
	"context"
	"strconv"
	"sync"
	"time"
)

type memoryUser struct {
	user         User
	passwordHash string
}

type MemoryRepository struct {
	users   map[string]memoryUser
	mutex   sync.RWMutex
	counter uint64
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		users: make(map[string]memoryUser),
	}
}

func (mr *MemoryRepository) Create(ctx context.Context, email string, passwordHash string) (User, error) {
	if err := ctx.Err(); err != nil {
		return User{}, err
	}

	mr.mutex.Lock()
	defer mr.mutex.Unlock()

	if err := ctx.Err(); err != nil {
		return User{}, err
	}

	if _, exists := mr.users[email]; exists {
		return User{}, ErrEmailAlreadyExists
	}

	mr.counter++

	createdUser := User{
		ID:        strconv.FormatUint(mr.counter, 10),
		Email:     email,
		CreatedAt: time.Now().UTC(),
	}

	mr.users[email] = memoryUser{
		user:         createdUser,
		passwordHash: passwordHash,
	}

	return createdUser, nil
}
