package errors

import "fmt"

type Service string
type Source string

const (
	ServiceUserSvc Service = "user-svc"

	SourcePostgres Source = "postgres"
	SourceRedis    Source = "redis"
	SourceKeys     Source = "keys"
	SourceServer   Source = "server"
)

type ServiceError struct {
	Service Service
	Source  Source
	Msg     string
	Err     error
}

func (e *ServiceError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] (%s): %s: %v", e.Service, e.Source, e.Msg, e.Err)
	}
	return fmt.Sprintf("[%s] (%s): %s", e.Service, e.Source, e.Msg)
}

// Wrap — внутренняя фабрика для обёртки ошибок
func Wrap(service Service, source Source, msg string, err error) error {
	if err == nil {
		return nil
	}
	return &ServiceError{
		Service: service,
		Source:  source,
		Msg:     msg,
		Err:     err,
	}
}

// New — создать ошибку без вложенного err
func New(service Service, source Source, msg string) error {
	return &ServiceError{
		Service: service,
		Source:  source,
		Msg:     msg,
	}
}
