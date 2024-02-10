package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/slack-go/slack"
)

func main() {

	// Environment Variables
	token := os.Getenv("KICKBOT_TOKEN")
	signingSecret := os.Getenv("KICKBOT_SIGNING_SECRET")
	envPort := os.Getenv("KICKBOT_PORT")

	// Flags
	port := flag.String("port", "4000", "Define the port on which the server will listen")
	flag.Parse()
	if envPort != "" {
		*port = envPort
	}

	// Game Manager
	gameMgr := NewGameManager(slack.New(token), DEFAULT_GAMEREQ_TIMEOUT)
	// Routes
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(SlackVerifyMiddleware(signingSecret))

	r.HandleFunc("/commands", handleSlackCommand(gameMgr))
	r.HandleFunc("/events", handleSlackEvent(gameMgr))

	// Server
	srv := &http.Server{
		Addr:           fmt.Sprintf(":%s", *port),
		Handler:        r,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		slog.Info(fmt.Sprintf("Server running on port %s", *port))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	shutdownSignal := <-shutdownChan

	log.Printf("Shutdown signal (%s) received, shutting down gracefully...\n", shutdownSignal)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("HTTP Server failed to shutdown gracefully", "error", err.Error())
	}
	slog.Info("HTTP Server successfully shutdown")
	gameMgr.Shutdown(ctx)
	slog.Info("Game Manager successfully shutdown")
	slog.Info("Shutdown complete. Server exiting.")
}
