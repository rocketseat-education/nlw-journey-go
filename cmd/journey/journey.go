package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/klauspost/compress/gzhttp"
	"github.com/phenpessoa/gutils/netutils/httputils"
	"github.com/phenpessoa/rocketseat-journey/internal/api"
	"github.com/phenpessoa/rocketseat-journey/internal/api/spec"
	"github.com/phenpessoa/rocketseat-journey/internal/mailpit"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGQUIT)
	defer cancel()

	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "something went wrong: %s\n", err)
		os.Exit(1)
	}

	fmt.Fprintln(os.Stdout, "all systems offline, exiting...")
}

func run(ctx context.Context) (err error) {
	var env string
	flag.StringVar(&env, "env", "prd", "either prd or dev, to set the environment")
	flag.Parse()

	var l *zap.Logger
	switch strings.ToLower(env) {
	case "prd":
		l, err = zap.NewProduction()
		if err != nil {
			return err
		}
	case "dev":
		cfg := zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

		l, err = cfg.Build()
		if err != nil {
			return err
		}
	}

	l = l.Named("journey_logger")
	defer func() { _ = l.Sync() }()

	pool, err := pgxpool.New(ctx, fmt.Sprintf(
		"user=%s password=%s host=%s port=%s dbname=%s",
		os.Getenv("JOURNEY_DATABASE_USER"), os.Getenv("JOURNEY_DATABASE_PASSWORD"), os.Getenv("JOURNEY_DATABASE_HOST"),
		os.Getenv("JOURNEY_DATABASE_PORT"), os.Getenv("JOURNEY_DATABASE_NAME"),
	))
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return err
	}

	api := api.NewAPI(pool, mailpit.New(pool), l)
	r := chi.NewMux()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(httputils.ChiLogger(l.Named("mux")))
	r.Mount("/", spec.Handler(api, spec.WithErrorHandler(api.ErrorHandlerFunc)))

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      gzhttp.GzipHandler(r),
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		ErrorLog:     zap.NewStdLog(l.Named("journey_http_error_logger")),
	}

	defer func() {
		const timeout = 30 * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		l.Info("Sending shutdown signal to HTTP server", zap.Duration("timeout", timeout))
		if shutdownErr := srv.Shutdown(ctx); shutdownErr != nil {
			l.Error("Failed to shutdown HTTP server", zap.Error(err))
			err = errors.Join(err, shutdownErr)
		}
		l.Info("HTTP Server shutdown")
	}()

	errChan := make(chan error, 1)

	go func() {
		l.Info("Journey is starting up")
		if err := srv.ListenAndServe(); err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				err = nil
			}
			errChan <- err
		}
	}()

	select {
	case <-ctx.Done():
		l.Info("Received shutdown signal, shutting down...")
	case err = <-errChan:
		if err != nil {
			l.Error("HTTP Server error, shutting down...", zap.Error(err))
		}
	}

	return err
}
