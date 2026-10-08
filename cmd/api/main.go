package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/earnmart/earnmart-be/internal/config"
	httpdelivery "github.com/earnmart/earnmart-be/internal/delivery/http"
	"github.com/earnmart/earnmart-be/internal/platform/database"
	mysqlrepo "github.com/earnmart/earnmart-be/internal/repository/mysql"
	"github.com/earnmart/earnmart-be/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load configuration", "error", err)
		os.Exit(1)
	}

	db, closeDB, err := database.NewMySQL(cfg.Database)
	if err != nil {
		logger.Error("connect to mysql", "error", err)
		os.Exit(1)
	}
	defer closeDB()

	userRepository := mysqlrepo.NewUserRepository(db)
	userService := service.NewUserService(userRepository, cfg.Security.BcryptCost)
	userHandler := httpdelivery.NewUserHandler(userService)
	router := httpdelivery.NewRouter(httpdelivery.RouterDependencies{
		AppName:     cfg.App.Name,
		Environment: cfg.App.Environment,
		CORSOrigins: cfg.HTTP.CORSAllowOrigins,
		DB:          db,
		UserHandler: userHandler,
	})

	server := &http.Server{
		Addr:         ":" + cfg.HTTP.Port,
		Handler:      router,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
		IdleTimeout:  cfg.HTTP.IdleTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("http server started", "address", server.Addr, "environment", cfg.App.Environment)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server failed", "error", err)
			os.Exit(1)
		}
		return
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("http server stopped")
}
