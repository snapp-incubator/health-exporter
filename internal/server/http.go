package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	klog "k8s.io/klog/v2"
)

type Server struct {
	srv *http.Server
}

func New(addr string, handler http.Handler) *Server {
	return &Server{
		srv: &http.Server{
			Addr:    addr,
			Handler: handler,
		},
	}
}

func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		klog.Infof("Listening on %s", s.srv.Addr)
		errCh <- s.srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
		err := <-errCh
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
