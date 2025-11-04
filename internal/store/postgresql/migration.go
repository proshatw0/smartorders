package postgresql

import (
	"fmt"
	"log"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// ApplyMigrations выполняет все миграции "вверх".
func ApplyMigrations(cfg Config) error {
	cfg.LoadFromEnv()

	m, err := migrate.New(
		fmt.Sprintf("file://%s", cfg.MigrationsDir),
		fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
			cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName, cfg.SSLMode),
	)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil {
		if err == migrate.ErrNoChange {
			log.Println("[migrate] no new migrations to apply")
			return nil
		}
		return fmt.Errorf("apply migrations: %w", err)
	}

	log.Println("[migrate] applied all migrations successfully")
	return nil
}

// RollbackMigrations откатывает на одну версию вниз.
func RollbackMigrations(cfg Config) error {
	cfg.LoadFromEnv()

	m, err := migrate.New(
		fmt.Sprintf("file://%s", cfg.MigrationsDir),
		fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
			cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName, cfg.SSLMode),
	)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer m.Close()

	if err := m.Steps(-1); err != nil {
		return fmt.Errorf("rollback migration: %w", err)
	}

	log.Println("[migrate] rolled back one migration successfully")
	return nil
}

// StatusMigrations показывает текущую версию миграций.
func StatusMigrations(cfg Config) error {
	cfg.LoadFromEnv()

	m, err := migrate.New(
		fmt.Sprintf("file://%s", cfg.MigrationsDir),
		fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
			cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName, cfg.SSLMode),
	)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer m.Close()

	version, dirty, err := m.Version()
	if err != nil {
		return fmt.Errorf("get migration version: %w", err)
	}

	state := "clean"
	if dirty {
		state = "dirty"
	}

	log.Printf("[migrate] current version: %d (%s)", version, state)
	return nil
}
