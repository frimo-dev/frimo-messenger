package email

import (
	"context"
	"log/slog"
)

type LogSender struct {
	logger *slog.Logger
}

func NewLogSender(logger *slog.Logger) *LogSender {
	return &LogSender{
		logger: logger,
	}
}

func (ls *LogSender) SendVerification(ctx context.Context, message VerificationMessage) error {
	ls.logger.Warn(
		"development email verification message",
		"recipient", message.Recipient,
		"verification_url", message.VerificationURL,
	)
	return nil
}

var _ Sender = (*LogSender)(nil)
