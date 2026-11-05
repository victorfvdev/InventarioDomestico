package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/estoque/api/internal/http/handlers"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func NewRouter(api *handlers.API, corsOrigins string) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(120 * time.Second))

	origins := strings.Split(corsOrigins, ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Get("/health", api.Health)
	r.Get("/api/session", api.GetSession)
	r.Get("/api/products", api.ListProducts)
	r.Post("/api/products/manual-entry", api.ManualEntry)
	r.Post("/api/products/{id}/adjust", api.InventoryAdjust)
	r.Get("/api/movements", api.ListMovements)
	r.Post("/api/vision/interpret", api.InterpretPhoto)
	r.Get("/api/vision/interpretations/{id}", api.GetInterpretation)
	r.Post("/api/vision/confirm-initial-load", api.ConfirmInitialLoad)

	return r
}
