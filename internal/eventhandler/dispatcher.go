package eventhandler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/frimo-dev/frimo-messenger/internal/email"
	"github.com/frimo-dev/frimo-messenger/internal/emailverification"
	"github.com/frimo-dev/frimo-messenger/internal/events"
	"github.com/frimo-dev/frimo-messenger/internal/outbox"
)

type TokenCipher interface {
	Decrypt(value []byte, additionalData []byte) ([]byte, error)
}

type Clock func() time.Time

type Dispatcher struct {
	emailSender        email.Sender
	deliveryRepository emailverification.DeliveryRepository
	tokenCipher        TokenCipher
	baseURL            string
	now                Clock
}

func NewDispatcher(
	emailSender email.Sender,
	deliveryRepository emailverification.DeliveryRepository,
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
	case events.EmailVerificationRequestedType:
		return d.handleEmailVerificationRequested(ctx, event)
	default:
		return fmt.Errorf("unsupported event type %q", event.Type)
	}
}

func (d *Dispatcher) handleEmailVerificationRequested(ctx context.Context, event outbox.Event) error {
	var payload events.EmailVerificationRequested

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("decode email verification event: %w", err)
	}

	if payload.VerificationID == "" {
		return errors.New("verification ID is empty")
	}

	if payload.Recipient == "" {
		return errors.New("verification recipient is empty")
	}

	data, err := d.deliveryRepository.GetForDelivery(ctx, payload.VerificationID)
	if err != nil {
		if errors.Is(err, emailverification.ErrDeliveryInactive) {
			// Уже подтверждённый или отозванный токен: повторная доставка не требуется
			return nil
		}

		return fmt.Errorf("get verification delivery data: %w", err)
	}

	// Истёкшее письмо уже нет смысла отправлять
	if !d.now().UTC().Before(data.ExpiresAt) {
		return nil
	}

	rawToken, err := d.tokenCipher.Decrypt(data.TokenCiphertext, []byte(data.ID))
	if err != nil {
		return fmt.Errorf("decrypt verification token: %w", err)
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
		return fmt.Errorf("send verification email: %w", err)
	}

	return nil
}
