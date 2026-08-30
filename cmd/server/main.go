package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/frimo-dev/frimo-messenger/internal/config"
	"github.com/frimo-dev/frimo-messenger/internal/handler/httpapi"
	"github.com/frimo-dev/frimo-messenger/internal/postgres"
	"github.com/frimo-dev/frimo-messenger/internal/security/password"
	"github.com/frimo-dev/frimo-messenger/internal/security/secret"
	"github.com/frimo-dev/frimo-messenger/internal/security/token"
	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
	"go.uber.org/zap"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed config loading", zap.Error(err))
	}

	databaseContext, cancelDatabase := context.WithTimeout(context.Background(), 5*time.Second)
	databasePool, err := postgres.Open(databaseContext, cfg.Database.URL)
	cancelDatabase()

	if err != nil {
		logger.Fatal("failed open database", zap.Error(err))
	}

	defer databasePool.Close()
	logger.Info("database connection established")

	verificationTokenCipher, err := secret.NewCipher(cfg.Auth.Verification.EncryptionKey)
	if err != nil {
		logger.Fatal("failed creation verification token cipher", zap.Error(err))
	}

	jwtIssuer := token.NewJWTManager(cfg.Auth.Login.AccessTokenSecret, cfg.Auth.Login.AccessTokenTTL)
	authRepository := postgres.NewAuthRepository(databasePool)
	passwordManager := password.NewArgon2Manager()
	tokenGenerator := token.NewGenerator()
	authService := auth.NewService(authRepository, jwtIssuer, passwordManager, tokenGenerator, verificationTokenCipher, time.Now, cfg.VerificationTokenLifetime)

	api := httpapi.New(logger, authService)

	server := &http.Server{
		Addr:              cfg.HTTP.Address,
		Handler:           api.Handler(),
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverError := make(chan error, 1)

	go func() {
		logger.Info("Server is listening", zap.String("port", server.Addr))

		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverError <- err
		}
	}()

	select {
	case err := <-serverError:
		logger.Fatal("server failed", zap.Error(err))
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), cfg.GracefulShutdown.HTTPShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownContext); err != nil {
		logger.Error("failed graceful shutdown failed", zap.Error(err))

		if closeErr := server.Close(); closeErr != nil {
			logger.Error("failed forced server close failed", zap.Error(closeErr))
		}
	}
}
