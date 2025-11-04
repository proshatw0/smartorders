package postgresql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
)

// Connect устанавливает соединение с PostgreSQL по указанной конфигурации.
// Возвращает готовый *sql.DB с проверенным подключением.
func Connect(ctx context.Context, cfg Config) (*sql.DB, error) {
	cfg.LoadFromEnv()

	dsn := cfg.DSN()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	// Настраиваем пул соединений (опционально)
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)

	// Проверим подключение с retry
	const (
		maxAttempts = 10
		delay       = 500 * time.Millisecond
	)
	for i := 1; i <= maxAttempts; i++ {
		if err = db.PingContext(ctx); err == nil {
			break
		}
		if i == maxAttempts {
			_ = db.Close()
			return nil, fmt.Errorf("connect to postgres failed after %d attempts: %w", i, err)
		}
		log.Printf("[postgresql] connection failed (attempt %d/%d): %v", i, maxAttempts, err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay * time.Duration(i)):
		}
	}

	log.Printf("[postgresql] connected to %s:%s/%s", cfg.Host, cfg.Port, cfg.DBName)

	// Применяем server-side параметры, если заданы
	if err := applySessionParams(ctx, db, cfg); err != nil {
		_ = db.Close()
		return nil, err
	}

	// Запускаем миграции при необходимости
	if cfg.RunMigrationsOnStart {
		if err := runMigrations(cfg); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("run migrations: %w", err)
		}
	}

	return db, nil
}

// applySessionParams задаёт statement_timeout и search_path, если они указаны в конфиге.
func applySessionParams(ctx context.Context, db *sql.DB, cfg Config) error {
	if cfg.StatementTimeout > 0 {
		ms := int(cfg.StatementTimeout.Milliseconds())
		if _, err := db.ExecContext(ctx, fmt.Sprintf("SET statement_timeout = %d", ms)); err != nil {
			return fmt.Errorf("set statement_timeout: %w", err)
		}
	}
	if cfg.Schema != "" && cfg.Schema != "public" {
		if _, err := db.ExecContext(ctx, fmt.Sprintf("SET search_path = %s", cfg.Schema)); err != nil {
			return fmt.Errorf("set search_path: %w", err)
		}
	}
	return nil
}

// runMigrations — простая заглушка для вызова мигратора.
// Позже можно подключить "github.com/golang-migrate/migrate/v4".
func runMigrations(cfg Config) error {
	if cfg.MigrationsDir == "" {
		return errors.New("no migrations directory specified")
	}
	log.Printf("[postgresql] migrations: %s (skipped — implement later)", cfg.MigrationsDir)
	return nil
}
