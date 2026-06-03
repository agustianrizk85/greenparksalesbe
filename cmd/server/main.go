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

	"greenpark/sales/internal/auth"
	"greenpark/sales/internal/config"
	"greenpark/sales/internal/repository"
	"greenpark/sales/internal/service"
	httptransport "greenpark/sales/internal/transport/http"
)

func main() {
	cfg := config.Load()

	// Dependency wiring (composition root).
	repo, err := repository.NewRepository(cfg.DataPath)
	if err != nil {
		log.Fatalf("sales: failed to open data store %q: %v", cfg.DataPath, err)
	}
	svc := service.New(repo)
	authSvc := auth.New(repo, cfg.SessionTTL)
	handler := httptransport.NewHandler(svc, authSvc)
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
