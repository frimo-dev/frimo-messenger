package emailverification

import (
	"context"
	"errors"
	"time"
)

var (
	ErrDeliveryNotFound = errors.New("email verification delivery not found")
	ErrDeliveryInactive = errors.New("email verification delivery is inactive")
)

type DeliveryData struct {
	ID              string
	TokenCiphertext []byte
	ExpiresAt       time.Time
}

type DeliveryRepository interface {
	GetForDelivery(ctx context.Context, verificationID string) (DeliveryData, error)
}
