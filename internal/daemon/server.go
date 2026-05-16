package daemon

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"time"

	"github.com/fridiculous/the-score/internal/api"
	"github.com/fridiculous/the-score/internal/model"
	"github.com/fridiculous/the-score/internal/runtime"
	"github.com/fridiculous/the-score/internal/store"
)

type Server struct {
	store   *store.Store
	handler *api.Handler
	logger  *slog.Logger
}

func New(logger *slog.Logger) *Server {
	st := store.New()
	seedSources(st)
	return &Server{
		store:   st,
		handler: api.NewHandler(st, runtime.SystemProcessLister{}),
		logger:  logger,
	}
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
	sources := []model.Source{
		{ID: "native", Name: "Native Observation API", Kind: "api", Enabled: true, Status: "active", SupportLevel: "native", Capabilities: []string{"sessions", "workspaces", "lineage", "events"}, ObservedAt: now},
		{ID: "process", Name: "Process Table", Kind: "runtime", Enabled: true, Status: "active", SupportLevel: "compatible", Capabilities: []string{"processes"}, ObservedAt: now},
		{ID: "git-worktree", Name: "Git Worktree", Kind: "workspace", Enabled: true, Status: "planned", SupportLevel: "not_installed", Capabilities: []string{"workspaces"}, Diagnostics: []string{"workspace discovery is declared but not implemented in this build"}, ObservedAt: now},
		{ID: "tmux", Name: "tmux", Kind: "runtime", Enabled: true, Status: "planned", SupportLevel: "not_installed", Capabilities: []string{"processes", "sessions"}, Diagnostics: []string{"tmux integration is declared but not implemented in this build"}, ObservedAt: now},
		{ID: "claude", Name: "Claude Code", Kind: "session", Enabled: true, Status: "partial", SupportLevel: "process_probe", Capabilities: []string{"sessions", "processes"}, Diagnostics: []string{"passive process detection is active; deeper Claude session telemetry is not implemented in this build"}, ObservedAt: now},
		{ID: "codex", Name: "Codex", Kind: "session", Enabled: true, Status: "partial", SupportLevel: "process_probe", Capabilities: []string{"sessions", "processes"}, Diagnostics: []string{"passive process detection is active; deeper Codex session telemetry is not implemented in this build"}, ObservedAt: now},
		{ID: "opencode", Name: "OpenCode", Kind: "session", Enabled: true, Status: "partial", SupportLevel: "process_probe", Capabilities: []string{"sessions", "processes"}, Diagnostics: []string{"passive process detection is active; deeper OpenCode session telemetry is not implemented in this build"}, ObservedAt: now},
		{ID: "hermes", Name: "Hermes", Kind: "session", Enabled: true, Status: "partial", SupportLevel: "process_probe", Capabilities: []string{"sessions", "processes"}, Diagnostics: []string{"passive process detection is active; deeper Hermes session telemetry is not implemented in this build"}, ObservedAt: now},
		{ID: "openclaw", Name: "OpenClaw", Kind: "session", Enabled: true, Status: "partial", SupportLevel: "process_probe", Capabilities: []string{"sessions", "processes"}, Diagnostics: []string{"passive process detection is active; deeper OpenClaw session telemetry is not implemented in this build"}, ObservedAt: now},
		{ID: "nanoclaw", Name: "NanoClaw", Kind: "session", Enabled: true, Status: "partial", SupportLevel: "process_probe", Capabilities: []string{"sessions", "processes"}, Diagnostics: []string{"passive process detection is active; deeper NanoClaw session telemetry is not implemented in this build"}, ObservedAt: now},
		{ID: "mcp", Name: "MCP", Kind: "protocol", Enabled: true, Status: "planned", SupportLevel: "not_installed", Capabilities: []string{"tool_calls", "events"}, Diagnostics: []string{"MCP source is part of the core bundle but not implemented in this build"}, ObservedAt: now},
	}
	for _, source := range sources {
		st.UpsertSource(source)
	}
}
