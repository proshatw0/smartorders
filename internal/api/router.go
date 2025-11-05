package api

import (
	"database/sql"
	"log"
	"net/http"
	"time"

	"smartorders/user-svc/internal/router"
)

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
	log.Print("HealthHandler")
}

func RegisterStub(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write([]byte(`{"message":"registered (stub)"}`))
}

func LoginStub(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write([]byte(`{"access_token":"stub-access","refresh_token":"stub-refresh"}`))
}

func NewRouter(db *sql.DB) http.Handler {
	r := router.New()

	r.Use(
		router.RequestID,
		router.Recover,
		router.Logger,
		router.CORS(),
	)

	r.Get("/health", HealthHandler)

	r.Route("/auth", func(r *router.Router) {
		r.Post("/register", RegisterStub)
		r.Post("/login", LoginStub)
	})

	return r.Timeout(15*time.Second, "request timeout")
}
