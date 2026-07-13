package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/frimo-dev/frimo-messenger/internal/config"
	"github.com/frimo-dev/frimo-messenger/internal/httpapi"
	"github.com/frimo-dev/frimo-messenger/internal/password"
	"github.com/frimo-dev/frimo-messenger/internal/postgres"
	"github.com/frimo-dev/frimo-messenger/internal/user"
)

func main() {
	cfg, err := config.Load()

	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	databaseContext, cancelDatabase := context.WithTimeout(context.Background(), 5*time.Second)
	databasePool, err := postgres.Open(databaseContext, cfg.Database.URL)
	cancelDatabase()

	if err != nil {
		log.Fatalf("open database: %w", err)
	}

	defer databasePool.Close()

	log.Println("database connection established")

	userRepository := user.NewMemoryRepository()
	passwordHasher := password.NewArgon2Hasher()
	userService := user.NewService(userRepository, passwordHasher)

	api := httpapi.New(userService)

	server := &http.Server{
		Addr:              cfg.HTTP.Address,
		Handler:           api.Handler(),
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	serverError := make(chan error, 1)

	go func() {
		log.Printf("Server is listening on: %s", server.Addr)

		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverError <- err
		}
	}()

	select {
	case err := <-serverError:
		log.Fatalf("server failed: %v", err)
	case <-ctx.Done():
		log.Println("shutdown signal received")
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownContext); err != nil {
		log.Printf("graceful shutdown failed: %v", err)

		if closeErr := server.Close(); closeErr != nil {
			log.Printf("forced server close failed: %v", closeErr)
		}
	}
}
