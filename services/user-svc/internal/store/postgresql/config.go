package postgresql

import (
	"fmt"
	"os"
	"time"
)

// Config — параметры подключения к PostgreSQL.
// USER_SVC_DB_USER, USER_SVC_DB_PASSWORD читаются из переменных окружения:
type Config struct {
	Host     string // адрес сервера
	Port     string // порт PostgreSQL
	DBName   string // имя базы данных
	User     string // имя пользователя (ENV::USER_SVC_DB_USER)
	Password string // пароль (ENV::USER_SVC_DB_PASSWORD)
	SSLMode  string // disable / require / verify-ca / verify-full

	AppName string // application_name в соединении
	Schema  string // целевая схема (опционально)

	ConnectTimeout   time.Duration // таймаут подключения
	StatementTimeout time.Duration // statement_timeout (server-side)
	LogSQL           bool          // логировать SQL-запросы
	LogLevel         string        // silent, error, warn, info, debug
	MetricsEnabled   bool          // включить метрики (например, через prometheus)

	RunMigrationsOnStart bool   // автоматически применять миграции при старте
	MigrationsDir        string // путь к миграциям
}

// DefaultConfig возвращает значения по умолчанию без секретов.
func DefaultConfig() Config {
	return Config{
		Host:                 "127.0.0.1",
		Port:                 "5432",
		DBName:               "user_svc",
		User:                 "", // (ENV::USER_SVC_DB_USER)
		Password:             "", // (ENV::USER_SVC_DB_PASSWORD)
		SSLMode:              "disable",
		AppName:              "user-svc",
		Schema:               "public",
		ConnectTimeout:       5 * time.Second,
		StatementTimeout:     0, // не задавать
		LogSQL:               false,
		LogLevel:             "info",
		MetricsEnabled:       true,
		RunMigrationsOnStart: false,
		MigrationsDir:        "./db/migrations",
	}
}

// LoadFromEnv подставляет USER и PASSWORD из окружения.
func (c *Config) LoadFromEnv() {
	if v := os.Getenv("USER_SVC_DB_USER"); v != "" {
		c.User = v
	}
	if v := os.Getenv("USER_SVC_DB_PASSWORD"); v != "" {
		c.Password = v
	}
}

// DSN возвращает готовую строку подключения для стандартного драйвера Go.
// Пример: "host=127.0.0.1 port=5432 user=postgres password=pass dbname=test sslmode=disable"
func (c Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s connect_timeout=%d application_name=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode, int(c.ConnectTimeout.Seconds()), c.AppName,
	)
}
