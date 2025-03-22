package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fibratus/portal/internal/api"
	"github.com/fibratus/portal/internal/config"
	"github.com/fibratus/portal/internal/db"
	"github.com/sirupsen/logrus"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "/etc/fibratus/server.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		logrus.Fatalf("Failed to load configuration: %v", err)
	}

	// Set up logging
	logLevel, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		logrus.Warnf("Invalid log level %s, defaulting to info", cfg.LogLevel)
		logLevel = logrus.InfoLevel
	}
	logrus.SetLevel(logLevel)
	logrus.SetFormatter(&logrus.JSONFormatter{})

	// Connect to database
	database, err := db.Connect(cfg.Database)
	if err != nil {
		logrus.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	// Run database migrations
	if err := db.Migrate(cfg.Database); err != nil {
		logrus.Fatalf("Failed to run database migrations: %v", err)
	}

	// Create API router
	router := api.NewRouter(cfg, database)

	// Configure HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      router,
	}

	// Start server in a goroutine
	go func() {
		logrus.Infof("Starting server on %s:%d", cfg.Server.Host, cfg.Server.Port)
		if cfg.Server.TLS.Enabled {
			if err := srv.ListenAndServeTLS(cfg.Server.TLS.CertFile, cfg.Server.TLS.KeyFile); err != nil {
				if err != http.ErrServerClosed {
					logrus.Fatalf("Failed to start server: %v", err)
				}
			}
		} else {
			logrus.Warn("TLS is disabled. It is highly recommended to enable TLS in production.")
			if err := srv.ListenAndServe(); err != nil {
				if err != http.ErrServerClosed {
					logrus.Fatalf("Failed to start server: %v", err)
				}
			}
		}
	}()

	// Set up graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	
	// Block until we receive our signal
	<-c
	
	// Create a context with a timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()
	
	// Shutdown the server
	srv.Shutdown(ctx)
	
	logrus.Info("Server gracefully shutdown")
}