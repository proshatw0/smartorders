package postgresql

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
)

var (
	db     *sql.DB
	dbOnce sync.Once
)

// Init глобально инициализирует подключение к PostgreSQL.
// Многократные вызовы возвращают одно и то же соединение.
func Init(ctx context.Context, cfg Config) (*sql.DB, error) {
	var err error
	dbOnce.Do(func() {
		var e error
		conn, e := Connect(ctx, cfg)
		if e != nil {
			err = fmt.Errorf("connect to postgres: %w", e)
			return
		}
		db = conn
	})
	return db, err
}

// Get возвращает уже инициализированное подключение.
func Get() *sql.DB {
	return db
}

// Close безопасно закрывает подключение.
func Close() error {
	if db != nil {
		return db.Close()
	}
	return nil
}
