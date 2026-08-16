package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/reserve/patrol-dispatch/internal/config"
	"github.com/reserve/patrol-dispatch/internal/service"
	"github.com/reserve/patrol-dispatch/internal/store"
	transporthttp "github.com/reserve/patrol-dispatch/internal/transport/http"
	"github.com/reserve/patrol-dispatch/internal/worker"
)

func main() {
	configPath := os.Getenv("PATROL_CONFIG")
	if configPath == "" {
		configPath = "config.json"
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Printf("warning: loading config from %s: %v — using defaults", configPath, err)
		cfg = config.Default()
	}

	st := store.New()
	ds := service.NewDispatchService(st)
	rs := service.NewResourceService(st)
	ms := service.NewMaterialService(st)
	ss := service.NewSyncService(st, ds)

	handler := transporthttp.NewHandler(ds, rs, ms, ss, cfg)
	router := handler.NewRouter()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	escWorker := worker.NewEscalationWorker(ds, st, cfg)
	go escWorker.Run(ctx)

	syncWorker := worker.NewSyncReplayWorker(ss, cfg)
	go syncWorker.Run(ctx)

	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("patrol-dispatch service listening on %s", cfg.ListenAddr)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("forced shutdown: %v", err)
	}
	log.Println("server stopped")
}
