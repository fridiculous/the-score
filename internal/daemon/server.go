package daemon

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"time"

	"github.com/fridiculous/the-score/internal/api"
	"github.com/fridiculous/the-score/internal/runtime"
	"github.com/fridiculous/the-score/internal/sources"
	"github.com/fridiculous/the-score/internal/store"
)

type Server struct {
	store   *store.Store
	handler *api.Handler
	logger  *slog.Logger
}

func New(logger *slog.Logger) (*Server, error) {
	st, err := store.NewSQLite("")
	if err != nil {
		return nil, err
	}
	seedSources(st)
	return &Server{
		store:   st,
		handler: api.NewHandler(st, runtime.SystemProcessLister{}),
		logger:  logger,
	}, nil
}

func NewWithStore(logger *slog.Logger, st *store.Store) *Server {
	seedSources(st)
	return &Server{
		store:   st,
		handler: api.NewHandler(st, runtime.SystemProcessLister{}),
		logger:  logger,
	}
}

func (s *Server) Close() error {
	if s.store == nil {
		return nil
	}
	return s.store.Close()
}

func (s *Server) SetShutdown(shutdown func()) {
	s.handler.SetShutdown(shutdown)
}

func (s *Server) Serve(ctx context.Context, listener net.Listener) error {
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				if errors.Is(err, net.ErrClosed) {
					return nil
				}
				return err
			}
		}
		go s.handler.ServeConn(ctx, conn)
	}
}

func seedSources(st *store.Store) {
	now := time.Now().UTC()
	for _, source := range sources.DefaultSources(now) {
		st.UpsertSource(source)
	}
}
