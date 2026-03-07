package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/neomody77/kuro/recovery/health"
	"github.com/neomody77/kuro/recovery/proxy"
	"github.com/neomody77/kuro/recovery/ui"
	"github.com/neomody77/kuro/recovery/version"
)

func main() {
	// Determine versions directory
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal("cannot determine home directory:", err)
	}
	versionsDir := filepath.Join(home, ".kuro", "versions")

	// Initialize version manager
	mgr := version.NewManager(versionsDir)
	if err := mgr.Scan(); err != nil {
		log.Fatal("failed to scan versions:", err)
	}

	// Start default version if one is set
	currentID := mgr.Current()
	if currentID != "" {
		log.Printf("Starting default version: %s", currentID)
		if err := mgr.Start(currentID); err != nil {
			log.Printf("Warning: failed to start default version %s: %v", currentID, err)
		}
	} else {
		log.Println("No default version set")
	}

	// Health checker: check every 10s, rollback after 3 consecutive failures
	checker := health.New(mgr, 10*time.Second, 3)
	checker.Start()

	// Admin UI server on :8081
	adminHandler := ui.New(mgr, checker)
	adminServer := &http.Server{
		Addr:    ":8081",
		Handler: adminHandler,
	}
	go func() {
		log.Println("Recovery admin starting on :8081")
		if err := adminServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("admin server error:", err)
		}
	}()

	// Reverse proxy on :8080
	proxyHandler := proxy.New(mgr)
	proxyServer := &http.Server{
		Addr:    ":8080",
		Handler: proxyHandler,
	}
	go func() {
		log.Println("Recovery proxy starting on :8080")
		if err := proxyServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("proxy server error:", err)
		}
	}()

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")

	checker.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	adminServer.Shutdown(ctx)
	proxyServer.Shutdown(ctx)

	mgr.StopAll()

	log.Println("Shutdown complete")
}
