// Command server starts the Sales (penjualan) CEO War-Room dashboard API.
//
// It wires the layers together — repository -> service -> HTTP transport — and
// runs an HTTP server with graceful shutdown.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"greenpark/sales/internal/ai"
	"greenpark/sales/internal/auth"
	"greenpark/sales/internal/authmw"
	"greenpark/sales/internal/config"
	"greenpark/sales/internal/gsheets"
	"greenpark/sales/internal/repository"
	"greenpark/sales/internal/service"
	httptransport "greenpark/sales/internal/transport/http"
)

func main() {
	cfg := config.Load()

	// Dependency wiring (composition root). Use PostgreSQL when a DSN is set,
	// otherwise persist to the JSON file.
	var (
		repo repository.SalesRepository
		err  error
	)
	if cfg.DatabaseURL != "" {
		repo, err = repository.NewPostgresRepository(cfg.DatabaseURL)
		if err != nil {
			log.Fatalf("sales: postgres: %v", err)
		}
		log.Println("sales: using PostgreSQL store")
	} else {
		repo, err = repository.NewRepository(cfg.DataPath)
		if err != nil {
			log.Fatalf("sales: failed to open data store %q: %v", cfg.DataPath, err)
		}
		log.Println("sales: using file store")
	}
	svc := service.New(repo)
	authSvc := auth.New(repo, cfg.SessionTTL)

	gs, err := gsheets.New(cfg.GoogleCreds)
	if err != nil {
		// A missing/unreadable credential must NOT take down the dashboard —
		// just disable the optional sync feature and keep serving.
		log.Printf("sales: Google Sheets sync disabled (kredensial tidak terbaca: %v)", err)
		gs = nil
	}
	if gs != nil {
		log.Printf("sales: Google Sheets sync enabled (sheet %s)", cfg.GoogleSheetID)
	} else {
		log.Println("sales: Google Sheets sync disabled (set SALES_GOOGLE_CREDENTIALS to enable)")
	}

	aiClient := ai.New(cfg.OpenRouterKey, cfg.OpenRouterModel, cfg.OpenRouterSite)
	if aiClient.Configured() {
		log.Printf("sales: AI alerts enabled (OpenRouter model %s)", cfg.OpenRouterModel)
	} else {
		log.Println("sales: AI alerts disabled (set OPENROUTER_API_KEY) — using rule-based alerts")
	}

	handler := httptransport.NewHandler(svc, authSvc, gs, cfg.GoogleSheetID, cfg.SyncIntervalM, aiClient)
	// Accept the unified dashboard's Ed25519 SSO login token directly (one login,
	// no token bridge) when AUTH_JWKS_URL is set. Native sales token still works.
	if v := authmw.New(authmw.Options{JWKSURL: os.Getenv("AUTH_JWKS_URL"), Issuer: os.Getenv("AUTH_ISSUER")}); v != nil {
		handler.SetSSO(v)
		log.Printf("sales: SSO token acceptance enabled (jwks=%s)", os.Getenv("AUTH_JWKS_URL"))
	}
	handler.StartAutoSync(context.Background())
	handler.StartRealtime() // WebSocket push on data change
	router := httptransport.NewRouter(handler, cfg.AllowOrigin)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Run the server in a goroutine so main can wait for shutdown signals.
	go func() {
		log.Printf("sales API listening on http://localhost:%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("sales: server error: %v", err)
		}
	}()

	// Wait for an interrupt signal for graceful shutdown.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("sales: shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("sales: graceful shutdown failed: %v", err)
	}
	log.Println("sales: stopped")
}
