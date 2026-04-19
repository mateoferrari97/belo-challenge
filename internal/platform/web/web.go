package web

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const (
	shutdownTimeout   = 10 * time.Second
	requestTimeout    = 30 * time.Second
	readHeaderTimeout = 5 * time.Second
)

type Server struct {
	mux *chi.Mux
}

func NewServer() *Server {
	mux := chi.NewMux()
	mux.Use(middleware.RequestID)
	mux.Use(middleware.Logger)
	mux.Use(middleware.Recoverer)
	mux.Use(middleware.Timeout(requestTimeout))

	s := &Server{mux: mux}
	s.mux.Method(http.MethodGet, "/ping", Handler(s.handlePing))
	return s
}

func (s *Server) Handle(method, pattern string, h http.Handler) {
	s.mux.Method(method, pattern, h)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) Run() error {
	if err := s.printRoutes(); err != nil {
		return err
	}

	return s.listenAndServe()
}

func (s *Server) handlePing(w http.ResponseWriter, _ *http.Request) error {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("pong"))
	return err
}

type route struct {
	method  string
	pattern string
}

func (s *Server) printRoutes() error {
	var routes []route
	walk := func(method, pattern string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routes = append(routes, route{method: method, pattern: pattern})
		return nil
	}
	if err := chi.Walk(s.mux, walk); err != nil {
		return fmt.Errorf("walking routes: %w", err)
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
	for _, r := range routes {
		if _, err := fmt.Fprintf(tw, "%s\t%s\n", r.method, r.pattern); err != nil {
			return fmt.Errorf("writing route: %w", err)
		}
	}
	return tw.Flush()
}

func (s *Server) listenAndServe() error {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           s,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	serverErrors := make(chan error, 1)

	go func() {
		log.Printf("API listening on %s", server.Addr)
		serverErrors <- server.ListenAndServe()
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		return fmt.Errorf("listen and serve: %w", err)
	case <-shutdown:
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		err := server.Shutdown(ctx)
		if err == nil {
			return nil
		}

		if err := server.Close(); err != nil {
			return fmt.Errorf("shutdown server: %w", err)
		}
	}

	return nil
}
