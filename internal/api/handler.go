package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/fridiculous/the-score/internal/model"
	"github.com/fridiculous/the-score/internal/runtime"
	"github.com/fridiculous/the-score/internal/store"
)

type Handler struct {
	store     *store.Store
	processes runtime.ProcessLister
	startedAt time.Time
	shutdown  func()
}

func NewHandler(st *store.Store, processes runtime.ProcessLister) *Handler {
	return &Handler{store: st, processes: processes, startedAt: time.Now().UTC()}
}

func (h *Handler) SetShutdown(shutdown func()) {
	h.shutdown = shutdown
}

func (h *Handler) ServeConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)
	for {
		var req Request
		if err := dec.Decode(&req); err != nil {
			if !errors.Is(err, io.EOF) {
				_ = enc.Encode(Response{
					JSONRPC: "2.0",
					Error:   &Error{Code: ErrCodeParseError, Message: err.Error()},
				})
			}
			return
		}
		if req.Method == "events/subscribe" {
			h.handleSubscribe(ctx, enc, req)
			return
		}
		result, rpcErr := h.Handle(req.Method, req.Params)
		resp := Response{JSONRPC: "2.0", ID: req.ID}
		if rpcErr != nil {
			resp.Error = rpcErr
		} else {
			resp.Result = result
		}
		if err := enc.Encode(resp); err != nil {
			return
		}
	}
}

func (h *Handler) Handle(method string, params json.RawMessage) (interface{}, *Error) {
	switch method {
	case "daemon/info":
		return map[string]interface{}{
			"name":      "The Score",
			"daemon":    "scored",
			"api":       "score-jsonrpc/v1",
			"pid":       os.Getpid(),
			"startedAt": h.startedAt,
		}, nil
	case "daemon/shutdown":
		if h.shutdown == nil {
			return nil, &Error{Code: ErrCodeInternal, Message: "daemon shutdown is not configured"}
		}
		go func() {
			time.Sleep(50 * time.Millisecond)
			h.shutdown()
		}()
		return map[string]bool{"shutdown": true}, nil
	case "health/check":
		return map[string]string{"status": "ok"}, nil
	case "sessions/list":
		var p struct {
			All       bool     `json:"all"`
			Status    []string `json:"status"`
			Workspace string   `json:"workspace"`
			Source    string   `json:"source"`
		}
		if err := decodeParams(params, &p); err != nil {
			return nil, invalidParams(err)
		}
		if err := h.reconcileProcessSessions(); err != nil {
			return nil, internalErr(err)
		}
		return h.store.ListSessions(store.SessionFilter{
			All:       p.All,
			Status:    p.Status,
			Workspace: p.Workspace,
			Source:    p.Source,
		}), nil
	case "sessions/get":
		var p struct {
			ID string `json:"id"`
		}
		if err := decodeParams(params, &p); err != nil {
			return nil, invalidParams(err)
		}
		session, err := h.store.GetSession(p.ID)
		if err != nil {
			return nil, notFound("session", p.ID)
		}
		return session, nil
	case "processes/list":
		processes, err := h.processes.ListProcesses()
		if err != nil {
			return nil, internalErr(err)
		}
		return processes, nil
	case "processes/get":
		var p struct {
			ID string `json:"id"`
		}
		if err := decodeParams(params, &p); err != nil {
			return nil, invalidParams(err)
		}
		processes, err := h.processes.ListProcesses()
		if err != nil {
			return nil, internalErr(err)
		}
		for _, proc := range processes {
			if proc.ID == p.ID {
				return proc, nil
			}
		}
		return nil, notFound("process", p.ID)
	case "workspaces/list":
		return h.store.ListWorkspaces(), nil
	case "workspaces/get":
		var p struct {
			ID string `json:"id"`
		}
		if err := decodeParams(params, &p); err != nil {
			return nil, invalidParams(err)
		}
		workspace, err := h.store.GetWorkspace(p.ID)
		if err != nil {
			return nil, notFound("workspace", p.ID)
		}
		return workspace, nil
	case "lineage/get":
		var p struct {
			SessionID string `json:"sessionId"`
		}
		if err := decodeParams(params, &p); err != nil {
			return nil, invalidParams(err)
		}
		return h.store.Lineage(p.SessionID), nil
	case "events/list":
		var p struct {
			Since int64 `json:"since"`
			Limit int   `json:"limit"`
		}
		if err := decodeParams(params, &p); err != nil {
			return nil, invalidParams(err)
		}
		return h.store.ListEvents(p.Since, p.Limit), nil
	case "sources/list":
		return h.store.ListSources(), nil
	case "sources/doctor":
		var p struct {
			ID string `json:"id"`
		}
		if err := decodeParams(params, &p); err != nil {
			return nil, invalidParams(err)
		}
		if p.ID == "" {
			return h.store.ListSources(), nil
		}
		source, err := h.store.GetSource(p.ID)
		if err != nil {
			return nil, notFound("source", p.ID)
		}
		return source, nil
	case "observations/upsertSession":
		var session model.Session
		if err := decodeParams(params, &session); err != nil {
			return nil, invalidParams(err)
		}
		if session.ID == "" {
			return nil, invalidParams(errors.New("id is required"))
		}
		return h.store.UpsertSession(session), nil
	case "observations/removeSession":
		var p struct {
			ID string `json:"id"`
		}
		if err := decodeParams(params, &p); err != nil {
			return nil, invalidParams(err)
		}
		return map[string]bool{"removed": h.store.RemoveSession(p.ID)}, nil
	case "observations/upsertWorkspace":
		var workspace model.Workspace
		if err := decodeParams(params, &workspace); err != nil {
			return nil, invalidParams(err)
		}
		if workspace.ID == "" && workspace.Path == "" {
			return nil, invalidParams(errors.New("id or path is required"))
		}
		return h.store.UpsertWorkspace(workspace), nil
	case "observations/upsertEdge":
		var edge model.Edge
		if err := decodeParams(params, &edge); err != nil {
			return nil, invalidParams(err)
		}
		if edge.From == "" || edge.To == "" || edge.Type == "" {
			return nil, invalidParams(errors.New("from, to, and type are required"))
		}
		return h.store.UpsertEdge(edge), nil
	default:
		return nil, &Error{Code: ErrCodeMethodNotFound, Message: "method not found: " + method}
	}
}

func (h *Handler) reconcileProcessSessions() error {
	processes, err := h.processes.ListProcesses()
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	seenEdges := map[string]bool{}
	pidToSession := map[int]string{}
	inferredProcesses := make([]model.Process, 0)
	now := time.Now().UTC()
	for _, proc := range processes {
		session, ok := runtime.InferAgentProcessSession(proc, now)
		if !ok {
			continue
		}
		if session.CWD == "" {
			if cwd, ok := runtime.LookupProcessCWD(proc.PID); ok {
				proc.CWD = cwd
				session.CWD = cwd
				session.WorkspaceRoots = []string{cwd}
				session.WorkspaceIDs = []string{"workspace:" + cwd}
			}
		}
		if existing, err := h.store.GetSession(session.ID); err == nil {
			if !isInferredProcessSession(existing) {
				continue
			}
			session.StartedAt = existing.StartedAt
		}
		seen[session.ID] = true
		pidToSession[proc.PID] = session.ID
		inferredProcesses = append(inferredProcesses, proc)
		h.store.UpsertSession(session)
		if session.CWD != "" {
			h.store.UpsertWorkspace(model.Workspace{
				ID:         "workspace:" + session.CWD,
				Kind:       "workspace",
				Path:       session.CWD,
				SessionIDs: []string{session.ID},
				Source:     session.Source,
				Confidence: model.ConfidenceLow,
				ObservedAt: now,
			})
		}
	}
	for _, proc := range inferredProcesses {
		parentID := pidToSession[proc.PPID]
		childID := pidToSession[proc.PID]
		if parentID == "" || childID == "" {
			continue
		}
		if child, err := h.store.GetSession(childID); err == nil && isInferredProcessSession(child) {
			child.ParentSessionID = parentID
			child.RootSessionID = parentID
			if parent, err := h.store.GetSession(parentID); err == nil && parent.RootSessionID != "" {
				child.RootSessionID = parent.RootSessionID
			}
			h.store.UpsertSession(child)
		}
		edgeID := "process:spawned_by:" + childID + "->" + parentID
		seenEdges[edgeID] = true
		h.store.UpsertEdge(model.Edge{
			ID:         edgeID,
			From:       childID,
			To:         parentID,
			Type:       "spawned_by",
			Source:     "process",
			Confidence: model.ConfidenceLow,
			ObservedAt: now,
		})
	}
	for _, session := range h.store.ListSessions(store.SessionFilter{All: true}) {
		if isInferredProcessSession(session) && !seen[session.ID] {
			h.store.RemoveSession(session.ID)
		}
	}
	for _, edge := range h.store.ListEdges() {
		if strings.HasPrefix(edge.ID, "process:spawned_by:") && !seenEdges[edge.ID] {
			h.store.RemoveEdge(edge.ID)
		}
	}
	return nil
}

func isInferredProcessSession(session model.Session) bool {
	processMeta, ok := session.Meta["process"].(map[string]interface{})
	if !ok {
		return false
	}
	inferred, _ := processMeta["inferred"].(bool)
	return inferred
}

func (h *Handler) handleSubscribe(ctx context.Context, enc *json.Encoder, req Request) {
	var p struct {
		Since int64 `json:"since"`
	}
	if err := decodeParams(req.Params, &p); err != nil {
		_ = enc.Encode(Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   invalidParams(err),
		})
		return
	}
	_, ch, cancel := h.store.Subscribe(p.Since)
	defer cancel()
	_ = enc.Encode(Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]interface{}{"subscribed": true},
	})
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			if err := enc.Encode(Notification{
				JSONRPC: "2.0",
				Method:  "events/event",
				Params:  event,
			}); err != nil {
				return
			}
		}
	}
}

func decodeParams(params json.RawMessage, target interface{}) error {
	if len(params) == 0 || string(params) == "null" {
		return nil
	}
	return json.Unmarshal(params, target)
}

func invalidParams(err error) *Error {
	return &Error{Code: ErrCodeInvalidParams, Message: err.Error()}
}

func internalErr(err error) *Error {
	return &Error{Code: ErrCodeInternal, Message: err.Error()}
}

func notFound(kind, id string) *Error {
	return &Error{Code: 404, Message: fmt.Sprintf("%s not found: %s", kind, id)}
}

func ParseStatusFilter(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
