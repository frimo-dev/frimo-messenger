package auth

import (
	"context"
	"errors"
	"time"
	"uuid"
)

var (
	ErrDeliveryNotFound = errors.New("email verification delivery not found")
	ErrDeliveryInactive = errors.New("email verification delivery is inactive")
)

type DeliveryData struct {
	ID              uuid.UUID
	TokenCiphertext []byte
	ExpiresAt       time.Time
}

type DeliveryRepository interface {
	GetForDelivery(ctx context.Context, verificationID uuid.UUID) (DeliveryData, error)
}
