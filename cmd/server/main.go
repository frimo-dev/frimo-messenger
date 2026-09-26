package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	postgres2 "github.com/frimo-dev/frimo-messenger/internal/adapter/repository/postgres"
	redis2 "github.com/frimo-dev/frimo-messenger/internal/adapter/repository/redis"
	"github.com/frimo-dev/frimo-messenger/internal/config"
	"github.com/frimo-dev/frimo-messenger/internal/handler/httpapi"
	"github.com/frimo-dev/frimo-messenger/internal/security/password"
	"github.com/frimo-dev/frimo-messenger/internal/security/secret"
	"github.com/frimo-dev/frimo-messenger/internal/security/token"
	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
	"github.com/frimo-dev/frimo-messenger/internal/service/user"
	"github.com/redis/go-redis/v9"
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
	databasePool, err := postgres2.Open(databaseContext, cfg.Database.URL)
	cancelDatabase()

	if err != nil {
		logger.Fatal("failed open database", zap.Error(err))
	}

	defer databasePool.Close()
	logger.Info("database connection established")

	cache, err := NewRedis(cfg.Cache)
	if err != nil {
		logger.Fatal("failed connection to redis", zap.Error(err))
	}
	defer func() {
		err = cache.Close()
		if err != nil {
			logger.Info("failed closing redis", zap.Error(err))
		}
	}()

	accessTokenStorage := redis2.NewAccessTokenStorage(cache)

	verificationTokenCipher, err := secret.NewCipher(cfg.Auth.Verification.EncryptionKey)
	if err != nil {
		logger.Fatal("failed creation verification token cipher", zap.Error(err))
	}

	//jwtManager := token.NewJWTManager(cfg.Auth.Login.AccessTokenSecret, cfg.Auth.Login.AccessTokenTTL)
	passwordManager := password.NewArgon2Manager()
	tokenGenerator := token.NewGenerator()

	authRepository := postgres2.NewAuthRepository(databasePool)
	sessionRepository := postgres2.NewSessionRepository(databasePool)

	// TODO: time.Hour * 24 * 30 into config
	authService := auth.NewService(
		authRepository,
		sessionRepository,
		accessTokenStorage,
		passwordManager,
		tokenGenerator,
		verificationTokenCipher,
		time.Now,
		cfg.VerificationTokenLifetime,
		cfg.Auth.Login.AccessTokenTTL,
		cfg.Auth.Login.RefreshTokenTTL,
	)

	userRepository := postgres2.NewUserRepository(databasePool)
	userService := user.NewService(userRepository)

	api := httpapi.New(logger, authService, userService)

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

func NewRedis(cfg config.RedisConfig) (*redis.Client, error) {
	cache := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := cache.Ping(ctx).Err(); err != nil {
		_ = cache.Close()
		return nil, fmt.Errorf("redis is unavailable: %w", err)
	}

	return cache, nil
}
