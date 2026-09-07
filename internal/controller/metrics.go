package controller

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	chimetrics "github.com/go-chi/metrics"
)

type MetricsServer struct {
	httpServer *http.Server
	shutdown   time.Duration
}

func NewMetricsServer(address string, shutdownTimeout time.Duration) *MetricsServer {
	if shutdownTimeout <= 0 {
		shutdownTimeout = 5 * time.Second
	}

	mux := http.NewServeMux()
	mux.Handle("GET /metrics", chimetrics.Handler())

	return &MetricsServer{
		httpServer: &http.Server{
			Addr:              address,
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
		},
		shutdown: shutdownTimeout,
	}
}

func (s *MetricsServer) Start(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	log.Printf("metrics server listening on %s", s.httpServer.Addr)

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.shutdown)
	defer cancel()

	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("metrics server graceful shutdown failed: %v\n", err)
		return err
	}
	return nil
}
