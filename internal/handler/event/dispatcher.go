package event

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/frimo-dev/frimo-messenger/internal/dto"
	"github.com/frimo-dev/frimo-messenger/internal/outbox"
	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
	"github.com/frimo-dev/frimo-messenger/internal/service/email"
)

type TokenCipher interface {
	Decrypt(value []byte, additionalData []byte) ([]byte, error)
}

type Clock func() time.Time

type Dispatcher struct {
	emailSender        email.Sender
	deliveryRepository auth.DeliveryRepository
	tokenCipher        TokenCipher
	baseURL            string
	now                Clock
}

func NewDispatcher(
	emailSender email.Sender,
	deliveryRepository auth.DeliveryRepository,
	tokenCipher TokenCipher,
	baseURL string,
	now Clock,
) *Dispatcher {
	if now == nil {
		now = time.Now
	}

	return &Dispatcher{
		emailSender:        emailSender,
		deliveryRepository: deliveryRepository,
		tokenCipher:        tokenCipher,
		baseURL:            strings.TrimRight(baseURL, "/"),
		now:                now,
	}
}

func (d *Dispatcher) Handle(ctx context.Context, event outbox.Event) error {
	switch event.Type {
	case dto.EmailVerificationRequestedType:
		return d.handleEmailVerificationRequested(ctx, event)
	default:
		return errors.Join(outbox.ErrNonRetryable, fmt.Errorf("unsupported event type %q", event.Type))
	}
}

func (d *Dispatcher) handleEmailVerificationRequested(ctx context.Context, event outbox.Event) error {
	var payload dto.EmailVerificationRequested

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return errors.Join(outbox.ErrNonRetryable, err)
	}

	if payload.VerificationID == "" {
		return errors.Join(outbox.ErrNonRetryable, errors.New("verification ID is empty"))
	}

	if payload.Recipient == "" {
		return errors.Join(outbox.ErrNonRetryable, errors.New("verification recipient is empty"))
	}

	data, err := d.deliveryRepository.GetForDelivery(ctx, payload.VerificationID)
	if err != nil {
		// Уже подтверждённый или отозванный токен: повторная доставка не требуется
		if errors.Is(err, auth.ErrDeliveryInactive) {
			return nil
		}

		return fmt.Errorf("failed to get verification delivery data: %w", err)
	}

	// Истёкшее письмо уже нет смысла отправлять
	if !d.now().UTC().Before(data.ExpiresAt) {
		return nil
	}

	rawToken, err := d.tokenCipher.Decrypt(data.TokenCiphertext, []byte(data.ID))
	if err != nil {
		return fmt.Errorf("failed to decrypt verification token: %w", err)
	}

	verificationURL := d.baseURL + "/auth/verify-email?token=" + url.QueryEscape(string(rawToken))

	err = d.emailSender.SendVerification(
		ctx,
		email.VerificationMessage{
			Recipient:       payload.Recipient,
			VerificationURL: verificationURL,
		},
	)

	if err != nil {
		return fmt.Errorf("failed to send verification email: %w", err)
	}

	return nil
}
