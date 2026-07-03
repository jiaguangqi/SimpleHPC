package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"simplehpc/backend/internal/config"
	"simplehpc/backend/internal/httpapi"
	"simplehpc/backend/internal/service"
)

func main() {
	cfg := config.Load()
	services, err := service.New(cfg)
	if err != nil {
		log.Fatalf("initialize services: %v", err)
	}
	defer services.Close()

	rootCtx, stopSync := context.WithCancel(context.Background())
	defer stopSync()
	services.StartSlurmJobSync(rootCtx)
	services.StartLDAPAccountSync(rootCtx)
	services.StartDashboardSampleSync(rootCtx)

	router := httpapi.NewRouter(cfg, services)
	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("simpleHPC backend listening on %s", cfg.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}
