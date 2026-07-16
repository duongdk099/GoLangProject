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
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	databaseCtx, cancelDatabase := context.WithTimeout(ctx, 10*time.Second)
	db, err := database.Open(databaseCtx, cfg.DatabaseURL)
	if err != nil {
		cancelDatabase()
		logger.Error("database unavailable", "error", err)
		os.Exit(1)
	}
	if err := database.Migrate(databaseCtx, db); err != nil {
		cancelDatabase()
		db.Close()
		logger.Error("database migration failed", "error", err)
		os.Exit(1)
	}
	cancelDatabase()
	defer db.Close()

	userService := users.NewService(users.NewPostgresStore(db))
	userHandler := users.NewHandler(userService)

	// pendingExchanges satisfies reviews.ExchangeLookup and
	// stats.ExchangeStatsProvider until Person 3's exchanges store exists.
	// See internal/exchanges/pending.go.
	pendingExchanges := exchanges.PendingIntegration{}

	serviceUseCases := services.NewUseCases(services.NewPostgresStore(db), userService)
	serviceHandler := services.NewHandler(serviceUseCases)

	reviewUseCases := reviews.NewUseCases(reviews.NewPostgresStore(db), pendingExchanges, serviceUseCases)
	reviewHandler := reviews.NewHandler(reviewUseCases)

	statsUseCases := stats.NewUseCases(stats.NewPostgresStore(db), pendingExchanges, userService)
	statsHandler := stats.NewHandler(statsUseCases)

	server := &http.Server{
		Addr: cfg.Address,
		Handler: httpapi.NewApplicationHandler(logger,
			userHandler, serviceHandler, reviewHandler, statsHandler,
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
