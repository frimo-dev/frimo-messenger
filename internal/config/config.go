package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	HTTP HTTPConfig
	Database DatabaseConfig
}

type DatabaseConfig struct {
	URL string
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
	}

	if cfg.HTTP.Address == "" {
		return Config{}, fmt.Errorf("HTTP_ADDR must not be empty")
	}

	if cfg.Database.URL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
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
