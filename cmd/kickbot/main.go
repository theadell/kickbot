package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"sync"
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
	gm := &GameManager{
		apiClient: slack.New(token),
		games:     make(map[SlackChannel]*Game),
		mu:        sync.Mutex{},
	}
	// Routes
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(SlackVerifyMiddleware(signingSecret))

	r.HandleFunc("/commands", handleSlackCommand(gm))
	r.HandleFunc("/events", handleSlackEvent(gm))

	// Server
	srv := &http.Server{
		Addr:           fmt.Sprintf(":%s", *port),
		Handler:        r,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	slog.Info(fmt.Sprintf("Server running on port %s", *port))
	log.Fatal(srv.ListenAndServe())
}
