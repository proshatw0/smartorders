package errors

// PG — ошибка уровня Postgres
func PG(msg string, err error) error {
	return Wrap(ServiceUserSvc, SourcePostgres, msg, err)
}

// Redis — ошибка уровня Redis
func Redis(msg string, err error) error {
	return Wrap(ServiceUserSvc, SourceRedis, msg, err)
}

// Keys — ошибка CLI/KeyStore
func Keys(msg string, err error) error {
	return Wrap(ServiceUserSvc, SourceKeys, msg, err)
}

// Server — ошибка HTTP/бизнес-логики
func Server(msg string, err error) error {
	return Wrap(ServiceUserSvc, SourceServer, msg, err)
}

// PGNew — вариант без вложенного err (например, "record not found")
func PGNew(msg string) error {
	return New(ServiceUserSvc, SourcePostgres, msg)
}

// RedisNew — без вложенного err
func RedisNew(msg string) error {
	return New(ServiceUserSvc, SourceRedis, msg)
}

// KeysNew — без вложенного err
func KeysNew(msg string) error {
	return New(ServiceUserSvc, SourceKeys, msg)
}

// ServerNew — без вложенного err
func ServerNew(msg string) error {
	return New(ServiceUserSvc, SourceServer, msg)
}
