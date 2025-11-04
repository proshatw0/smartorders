package app

import (
	"os"
	"time"
)

// Config — единый набор параметров запуска HTTP-сервера.
type Config struct {
	Host string // IP-адрес для привязки
	Port string // порт для HTTP-сервера

	LogFile string // путь к файлу логов (используется при -bg)
	PidFile string // путь к PID-файлу при фоновой работе

	ReadTimeout  time.Duration // максимальное время чтения запроса клиентом
	WriteTimeout time.Duration // максимальное время записи ответа клиенту
	IdleTimeout  time.Duration // максимальное время простоя соединения
}

// DefaultServerConfig возвращает дефолтные параметры конфигурации.
func DefaultServerConfig() Config {
	port := os.Getenv("USER_SVC_PORT")
	if port == "" {
		port = "8080"
	}
	return Config{
		Host:         "0.0.0.0",
		Port:         port,
		LogFile:      "./user-svc.log",
		PidFile:      "./user-svc.pid",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 20 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}
