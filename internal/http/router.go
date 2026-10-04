package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/nammmelur/melur-app/internal/health"
)

func NewRouter() *chi.Mux {
	router := chi.NewRouter()
	router.Get("/health", health.NewHandler().Health)
	return router
}
