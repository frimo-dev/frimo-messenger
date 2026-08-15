package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	HTTP             HTTPConfig
	Database         DatabaseConfig
	App              AppConfig
	Verification     VerificationConfig
	GracefulShutdown GracefulShutdownConfig

	VerificationTokenLifetime time.Duration
}

type AppConfig struct {
	BaseURL string
}

type DatabaseConfig struct {
	URL string
}

type VerificationConfig struct {
	EncryptionKey string
}

type HTTPConfig struct {
	Address string
	// Сколько сервер ждёт HTTP-заголовки
	ReadHeaderTimeout time.Duration
	// Сколько времени разрешено на чтение всего запроса
	ReadTimeout time.Duration
	// Сколько времени разрешено на формирование и отправку ответа
	WriteTimeout time.Duration
	// Сколько неактивное keep-alive соединение может оставаться открытым
	IdleTimeout time.Duration
}

type GracefulShutdownConfig struct {
	HTTPShutdownTimeout   time.Duration
	WorkerShutdownTimeout time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		HTTP: HTTPConfig{
			Address:           getEnv("HTTP_ADDR", ":8080"),
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
		Database: DatabaseConfig{
			URL: os.Getenv("DATABASE_URL"),
		},
		App: AppConfig{
			BaseURL: getEnv("APP_BASE_URL", "http://localhost:8080"),
		},
		Verification: VerificationConfig{
			EncryptionKey: os.Getenv(
				"VERIFICATION_TOKEN_ENCRYPTION_KEY",
			),
		},
		GracefulShutdown: GracefulShutdownConfig{
			HTTPShutdownTimeout:   10 * time.Second,
			WorkerShutdownTimeout: 10 * time.Second,
		},
		VerificationTokenLifetime: 30 * time.Minute,
	}

	if cfg.HTTP.Address == "" {
		return Config{}, fmt.Errorf("HTTP_ADDR must not be empty")
	}

	if cfg.Database.URL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	if cfg.App.BaseURL == "" {
		return Config{}, fmt.Errorf("APP_BASE_URL must not be empty")
	}

	if cfg.Verification.EncryptionKey == "" {
		return Config{}, fmt.Errorf(
			"VERIFICATION_TOKEN_ENCRYPTION_KEY is required",
		)
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		return fallback
	}

	return value
}
