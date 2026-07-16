package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := LoadConfig()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	databaseCtx, cancelDatabase := context.WithTimeout(ctx, 10*time.Second)
	db, err := OpenDatabase(databaseCtx, config.DatabaseURL)
	if err != nil {
		cancelDatabase()
		logger.Error("database unavailable", "error", err)
		os.Exit(1)
	}
	if err := Migrate(databaseCtx, db); err != nil {
		cancelDatabase()
		db.Close()
		logger.Error("database migration failed", "error", err)
		os.Exit(1)
	}
	cancelDatabase()
	defer db.Close()

	userStore := NewPostgresUserStore(db)
	userService := NewUserService(userStore)
	userHandler := NewUserHandler(userService)

	server := &http.Server{
		Addr:         config.Address,
		Handler:      NewApplicationHandler(logger, userHandler),
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		IdleTimeout:  config.IdleTimeout,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("HTTP server started", "address", config.Address)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server failed", "error", err)
		}
	case <-ctx.Done():
		logger.Info("shutdown requested")
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}
