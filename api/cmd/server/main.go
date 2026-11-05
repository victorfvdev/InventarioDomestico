package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/estoque/api/internal/config"
	"github.com/estoque/api/internal/db"
	"github.com/estoque/api/internal/http/handlers"
	httpapi "github.com/estoque/api/internal/http"
	"github.com/estoque/api/internal/nfce"
	"github.com/estoque/api/internal/repository"
	"github.com/estoque/api/internal/service"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	_ = godotenv.Load("../.env")
	_ = godotenv.Load("../../.env")

	cfg := config.Load()
	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("banco: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	repo := repository.New(pool)
	session, err := repo.EnsureBootstrap(ctx, cfg.HouseholdName, cfg.DefaultUserName)
	if err != nil {
		log.Fatalf("bootstrap: %v", err)
	}
	log.Printf("residência=%s usuário=%s", session.Household.Name, session.User.DisplayName)

	inventory := service.NewInventoryService(repo)
	vision := service.NewVisionService(repo, cfg.OpenAIAPIKey, cfg.VisionModel, cfg.UploadDir)
	nfceAdapter := nfce.NewAdapter(cfg.NFCeFixturePath, cfg.NFCeForceFixture)
	purchase := service.NewPurchaseService(repo, nfceAdapter)
	api := &handlers.API{
		Repo:           repo,
		Inventory:      inventory,
		Vision:         vision,
		Purchase:       purchase,
		CurrentSession: session,
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpapi.NewRouter(api, cfg.CORSOrigins),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("API Estoque ouvindo em http://127.0.0.1%s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
