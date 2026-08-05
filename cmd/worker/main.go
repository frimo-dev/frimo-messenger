package main

import (
	"context"
	"encoding/base64"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/frimo-dev/frimo-messenger/internal/config"
	"github.com/frimo-dev/frimo-messenger/internal/email"
	"github.com/frimo-dev/frimo-messenger/internal/eventhandler"
	"github.com/frimo-dev/frimo-messenger/internal/outbox"
	"github.com/frimo-dev/frimo-messenger/internal/postgres"
	"github.com/frimo-dev/frimo-messenger/internal/security/secret"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	rootContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	databaseContext, cancelDatabase := context.WithTimeout(rootContext, 5*time.Second)
	databasePool, err := postgres.Open(databaseContext, cfg.Database.URL)
	cancelDatabase()

	if err != nil {
		logger.Error("open database", "error", err)
		os.Exit(1)
	}
	defer databasePool.Close()

	encryptionKey, err := base64.StdEncoding.DecodeString(cfg.Verification.EncryptionKey)
	if err != nil {
		logger.Error(
			"decode verification encryption key",
			"error", err,
		)
		os.Exit(1)
	}

	tokenCipher, err := secret.NewCipher(encryptionKey)
	if err != nil {
		logger.Error(
			"create verification token cipher",
			"error", err,
		)
		os.Exit(1)
	}

	emailSender := email.NewLogSender(logger)

	verificationRepository := postgres.NewEmailVerificationRepository(databasePool)

	dispatcher := eventhandler.NewDispatcher(
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
		logger.Error("worker failed", "error", err)
		os.Exit(1)
	}

	logger.Info("worker stopped")
}
