package main

import (
	"context"
	"encoding/base64"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/frimo-dev/frimo-messenger/internal/config"
	"github.com/frimo-dev/frimo-messenger/internal/handler/event"
	"github.com/frimo-dev/frimo-messenger/internal/outbox"
	"github.com/frimo-dev/frimo-messenger/internal/postgres"
	"github.com/frimo-dev/frimo-messenger/internal/security/secret"
	"github.com/frimo-dev/frimo-messenger/internal/service/email"
	"go.uber.org/zap"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed config loading", zap.Error(err))
	}

	rootContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	databaseContext, cancelDatabase := context.WithTimeout(rootContext, 5*time.Second)
	databasePool, err := postgres.Open(databaseContext, cfg.Database.URL)
	cancelDatabase()

	if err != nil {
		logger.Fatal("failed open database", zap.Error(err))
	}
	defer databasePool.Close()

	encryptionKey, err := base64.StdEncoding.DecodeString(cfg.Verification.EncryptionKey)
	if err != nil {
		logger.Fatal("failed decode verification encryption key", zap.Error(err))
	}

	tokenCipher, err := secret.NewCipher(encryptionKey)
	if err != nil {
		logger.Fatal("failed creation verification token cipher", zap.Error(err))
	}

	emailSender := email.NewLogSender(logger)

	verificationRepository := postgres.NewEmailVerificationRepository(databasePool)

	dispatcher := event.NewDispatcher(
		emailSender,
		verificationRepository,
		tokenCipher,
		cfg.App.BaseURL,
		time.Now,
	)

	outboxRepository := postgres.NewOutboxRepository(databasePool)

	processor := outbox.NewProcessor(
		outboxRepository,
		dispatcher,
		logger,
		time.Now,
	)

	logger.Info("worker started")

	if err := processor.Run(rootContext); err != nil {
		logger.Fatal("worker failed", zap.Error(err))
	}

	logger.Info("worker stopped")
}
