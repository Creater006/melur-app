package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/nammmelur/melur-app/internal/auth"
	"github.com/nammmelur/melur-app/internal/health"
)

func NewRouter(authHandler *auth.Handler) *chi.Mux {
	router := chi.NewRouter()
	router.Get("/health", health.NewHandler().Health)
	router.Post("/api/v1/auth/login", authHandler.Login)
	router.Post("/api/v1/auth/logout", authHandler.Logout)
	router.Post("/api/v1/auth/forgot-password", authHandler.ForgotPassword)
	router.Post("/api/v1/auth/reset-password", authHandler.ResetPassword)
	return router
}
