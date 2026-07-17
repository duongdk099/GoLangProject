package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"barterswap/internal/config"
	"barterswap/internal/database"
	"barterswap/internal/exchanges"
	"barterswap/internal/reviews"
	"barterswap/internal/services"
	"barterswap/internal/stats"
	"barterswap/internal/users"
	"barterswap/pkg/httpapi"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, logger, config.Load()); err != nil {
		logger.Error("server terminated", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger, cfg config.Config) error {
	databaseCtx, cancelDatabase := context.WithTimeout(ctx, 10*time.Second)
	db, err := database.Open(databaseCtx, cfg.DatabaseURL)
	if err != nil {
		cancelDatabase()
		return fmt.Errorf("database unavailable: %w", err)
	}
	if err := database.Migrate(databaseCtx, db); err != nil {
		cancelDatabase()
		db.Close()
		return fmt.Errorf("database migration failed: %w", err)
	}
	cancelDatabase()
	defer db.Close()

	userService := users.NewService(users.NewPostgresStore(db))
	userHandler := users.NewHandler(userService)

	serviceUseCases := services.NewUseCases(services.NewPostgresStore(db), userService)
	serviceHandler := services.NewHandler(serviceUseCases)

	exchangeUseCases := exchanges.NewUseCases(exchanges.NewPostgresStore(db), serviceUseCases, userService)
	exchangeHandler := exchanges.NewHandler(exchangeUseCases)

	reviewUseCases := reviews.NewUseCases(reviews.NewPostgresStore(db), exchangeUseCases, serviceUseCases)
	reviewHandler := reviews.NewHandler(reviewUseCases)

	statsUseCases := stats.NewUseCases(stats.NewPostgresStore(db), exchangeUseCases, userService)
	statsHandler := stats.NewHandler(statsUseCases)

	server := &http.Server{
		Addr: cfg.Address,
		Handler: httpapi.NewApplicationHandler(logger,
			userHandler, serviceHandler, exchangeHandler, reviewHandler, statsHandler,
		),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("HTTP server started", "address", cfg.Address)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("HTTP server failed: %w", err)
		}
	case <-ctx.Done():
		logger.Info("shutdown requested")
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown failed: %w", err)
	}
	return nil
}
