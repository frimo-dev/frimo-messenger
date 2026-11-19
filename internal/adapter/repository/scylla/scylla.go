package scylla

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/frimo-dev/frimo-messenger/internal/config"
	"github.com/gocql/gocql"
)

func Open(ctx context.Context, cfg config.ScyllaDBConfig) (*gocql.Session, error) {
	if cfg.Host == "" {
		return nil, errors.New("scylla host is required")
	}

	if cfg.Keyspace == "" {
		return nil, errors.New("scylla keyspace is required")
	}

	if cfg.Port == 0 {
		cfg.Port = 9042
	}

	cluster := gocql.NewCluster(cfg.Host)
	cluster.Port = cfg.Port
	cluster.Keyspace = cfg.Keyspace

	// Максимальное время ожидания подключения и запроса.
	cluster.ConnectTimeout = 5 * time.Second
	cluster.Timeout = 10 * time.Second

	// Уровень согласованности запросов по умолчанию.
	cluster.Consistency = gocql.LocalQuorum

	// Направляем запросы к нодам, владеющим нужными данными.
	cluster.PoolConfig.HostSelectionPolicy = gocql.TokenAwareHostPolicy(gocql.RoundRobinHostPolicy())

	// Настраиваем авторизацию.
	if cfg.Username != "" {
		cluster.Authenticator = gocql.PasswordAuthenticator{Username: cfg.Username, Password: cfg.Password}
	}

	session, err := cluster.CreateSession()
	if err != nil {
		return nil, fmt.Errorf("create scylla session: %w", err)
	}

	// Проверяем, что можем выполнить CQL-запрос.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var releaseVersion string

	err = session.Query("SELECT release_version FROM system.local").WithContext(ctx).Scan(&releaseVersion)
	if err != nil {
		session.Close()

		return nil, fmt.Errorf("check scylla connection: %w", err)
	}

	return session, nil
}
