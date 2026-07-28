package email

import "context"

type VerificationMessage struct {
	Recipient       string
	VerificationURL string
}

type Sender interface {
	SendVerification(ctx context.Context, message VerificationMessage) error
}
