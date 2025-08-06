package app

// Config — единый набор параметров запуска сервера.
//
// Host, Port       — адрес, на котором слушает HTTP-сервер.
// LogFile          — путь к файлу, куда писать stdout/stderr в фоне.
// PidFile          — путь к PID-файлу.
type Config struct {
	Host    string
	Port    string
	LogFile string
	PidFile string
}

// DefaultServerConfig возвращает дефолтные значения конфигурации.
// Их можно переопределить через флаги или позиционные аргументы.
func DefaultServerConfig() Config {
	return Config{
		Host:    "0.0.0.0",
		Port:    "8001",
		LogFile: "./user-svc.log",
		PidFile: "./user-svc.pid",
	}
}
