package email

import (
	"context"

	"go.uber.org/zap"
)

type LogSender struct {
	logger *zap.Logger
}

func NewLogSender(logger *zap.Logger) *LogSender {
	return &LogSender{
		logger: logger,
	}
}

func (ls *LogSender) SendVerification(ctx context.Context, message VerificationMessage) error {
	ls.logger.Warn(
		"development email verification message",
		zap.String("recipient", message.Recipient),
		zap.String("verification_url", message.VerificationURL),
	)
	return nil
}

var _ Sender = (*LogSender)(nil)
