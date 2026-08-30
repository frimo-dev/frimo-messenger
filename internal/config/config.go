package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"time"
)

type Config struct {
	HTTP             HTTPConfig
	Database         DatabaseConfig
	App              AppConfig
	Auth             AuthConfig
	GracefulShutdown GracefulShutdownConfig

	VerificationTokenLifetime time.Duration
}

type AppConfig struct {
	BaseURL string
}

type DatabaseConfig struct {
	URL string
}

type AuthConfig struct {
	Login        LoginConfig
	Verification VerificationConfig
}

type LoginConfig struct {
	AccessTokenSecret []byte
	AccessTokenTTL    time.Duration
}

type VerificationConfig struct {
	EncryptionKey []byte
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
	encryptionKey, err := base64.StdEncoding.DecodeString(os.Getenv("VERIFICATION_TOKEN_ENCRYPTION_KEY"))
	if err != nil {
		return Config{}, fmt.Errorf("failed to decode VERIFICATION_TOKEN_ENCRYPTION_KEY: %v", err)
	}

	accessTokenSecret, err := base64.StdEncoding.DecodeString(os.Getenv("ACCESS_TOKEN_SECRET"))
	if err != nil {
		return Config{}, fmt.Errorf("failed to decode ACCESS_TOKEN_SECRET: %v", err)
	}

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
			BaseURL: getEnv("APP_BASE_URL", "http://localhost"),
		},
		Auth: AuthConfig{
			Login: LoginConfig{
				AccessTokenSecret: accessTokenSecret,
				AccessTokenTTL:    15 * time.Minute,
			},
			Verification: VerificationConfig{
				EncryptionKey: encryptionKey,
			},
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

	if len(cfg.Auth.Verification.EncryptionKey) != 32 {
		return Config{}, fmt.Errorf("VERIFICATION_TOKEN_ENCRYPTION_KEY must decode to 32 bytes")
	}

	if len(cfg.Auth.Login.AccessTokenSecret) != 32 {
		return Config{}, fmt.Errorf("ACCESS_TOKEN_SECRET must decode to 32 bytes")
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
