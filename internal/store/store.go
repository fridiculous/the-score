package store

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fridiculous/the-score/internal/model"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	mu          sync.RWMutex
	sessions    map[string]model.Session
	workspaces  map[string]model.Workspace
	edges       map[string]model.Edge
	sources     map[string]model.Source
	events      []model.Event
	nextSeq     int64
	subscribers map[int]chan model.Event
	nextSubID   int
	maxEvents   int
}

func New() *Store {
	return &Store{
		sessions:    make(map[string]model.Session),
		workspaces:  make(map[string]model.Workspace),
		edges:       make(map[string]model.Edge),
		sources:     make(map[string]model.Source),
		subscribers: make(map[int]chan model.Event),
		maxEvents:   5000,
	}
}

func (s *Store) UpsertSession(session model.Session) model.Session {
	now := time.Now().UTC()
	if session.ID == "" {
		return session
	}
	if session.Kind == "" {
		session.Kind = "session"
	}
	if session.Status == "" {
		session.Status = model.StatusUnknown
	}
	if session.Attention == "" {
		session.Attention = deriveAttention(session.Status)
	}
	if session.Confidence == "" {
		session.Confidence = model.ConfidenceUnknown
	}
	if session.Source == "" {
		session.Source = "native"
	}
	if session.LastActivityAt.IsZero() {
		session.LastActivityAt = now
	}
	if session.StartedAt.IsZero() {
		session.StartedAt = session.LastActivityAt
	}
	if session.RootSessionID == "" {
		if session.ParentSessionID != "" {
			session.RootSessionID = session.ParentSessionID
		} else {
			session.RootSessionID = session.ID
		}
	}

	s.mu.Lock()
	old, existed := s.sessions[session.ID]
	s.sessions[session.ID] = session
	s.mu.Unlock()

	eventType := "session.upserted"
	summary := "session upserted"
	if !existed {
		eventType = "session.started"
		summary = "session started"
	} else if old.Status != session.Status {
		eventType = "status.changed"
		summary = string(old.Status) + " -> " + string(session.Status)
	}
	s.appendEvent(model.Event{
		Type:      eventType,
		SessionID: session.ID,
		Summary:   summary,
		Source:    session.Source,
		Data: map[string]interface{}{
			"status":    session.Status,
			"attention": session.Attention,
		},
	})
	return session
}

func (s *Store) RemoveSession(id string) bool {
	s.mu.Lock()
	_, existed := s.sessions[id]
	if existed {
		delete(s.sessions, id)
	}
	s.mu.Unlock()
	if existed {
		s.appendEvent(model.Event{
			Type:      "session.removed",
			SessionID: id,
			Summary:   "session removed",
			Source:    "native",
		})
	}
	return existed
}

func (s *Store) GetSession(id string) (model.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[id]
	if !ok {
		return model.Session{}, ErrNotFound
	}
	return session, nil
}

type SessionFilter struct {
	All       bool
	Status    []string
	Workspace string
	Source    string
}

func (s *Store) ListSessions(filter SessionFilter) []model.Session {
	statuses := make(map[string]bool, len(filter.Status))
	for _, status := range filter.Status {
		if status != "" {
			statuses[status] = true
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.Session, 0, len(s.sessions))
	for _, session := range s.sessions {
		if !filter.All && isTerminal(session.Status) {
			continue
		}
		if len(statuses) > 0 && !statuses[string(session.Status)] {
			continue
		}
		if filter.Source != "" && session.Source != filter.Source && session.Agent.Adapter != filter.Source {
			continue
		}
		if filter.Workspace != "" && !sessionMatchesWorkspace(session, filter.Workspace) {
			continue
		}
		out = append(out, session)
	}
	sort.Slice(out, func(i, j int) bool {
		ai, aj := attentionRank(out[i]), attentionRank(out[j])
		if ai != aj {
			return ai < aj
		}
		return out[i].LastActivityAt.After(out[j].LastActivityAt)
	})
	return out
}

func (s *Store) UpsertWorkspace(workspace model.Workspace) model.Workspace {
	now := time.Now().UTC()
	if workspace.ID == "" {
		workspace.ID = "workspace:" + workspace.Path
	}
	if workspace.Kind == "" {
		workspace.Kind = "workspace"
	}
	if workspace.Source == "" {
		workspace.Source = "native"
	}
	if workspace.Confidence == "" {
		workspace.Confidence = model.ConfidenceUnknown
	}
	if workspace.ObservedAt.IsZero() {
		workspace.ObservedAt = now
	}
	s.mu.Lock()
	_, existed := s.workspaces[workspace.ID]
	s.workspaces[workspace.ID] = workspace
	s.mu.Unlock()
	eventType := "workspace.upserted"
	if !existed {
		eventType = "workspace.discovered"
	}
	s.appendEvent(model.Event{
		Type:        eventType,
		WorkspaceID: workspace.ID,
		Summary:     workspace.Path,
		Source:      workspace.Source,
	})
	return workspace
}

func (s *Store) GetWorkspace(id string) (model.Workspace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	workspace, ok := s.workspaces[id]
	if !ok {
		return model.Workspace{}, ErrNotFound
	}
	return workspace, nil
}

func (s *Store) ListWorkspaces() []model.Workspace {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.Workspace, 0, len(s.workspaces))
	for _, workspace := range s.workspaces {
		out = append(out, workspace)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Path < out[j].Path
	})
	return out
}

func (s *Store) UpsertEdge(edge model.Edge) model.Edge {
	now := time.Now().UTC()
	if edge.ID == "" {
		edge.ID = edge.Type + ":" + edge.From + "->" + edge.To
	}
	if edge.Source == "" {
		edge.Source = "native"
	}
	if edge.Confidence == "" {
		edge.Confidence = model.ConfidenceUnknown
	}
	if edge.ObservedAt.IsZero() {
		edge.ObservedAt = now
	}
	s.mu.Lock()
	_, existed := s.edges[edge.ID]
	s.edges[edge.ID] = edge
	s.mu.Unlock()
	eventType := "graph.edge_upserted"
	if !existed && (edge.Type == "spawned_by" || edge.Type == "delegated_by") {
		eventType = "child.spawned"
	}
	s.appendEvent(model.Event{
		Type:    eventType,
		EdgeID:  edge.ID,
		Summary: edge.Type + " " + edge.From + " -> " + edge.To,
		Source:  edge.Source,
	})
	return edge
}

func (s *Store) ListEdges() []model.Edge {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.Edge, 0, len(s.edges))
	for _, edge := range s.edges {
		out = append(out, edge)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ObservedAt.Before(out[j].ObservedAt)
	})
	return out
}

func (s *Store) RemoveEdge(id string) bool {
	s.mu.Lock()
	_, existed := s.edges[id]
	if existed {
		delete(s.edges, id)
	}
	s.mu.Unlock()
	if existed {
		s.appendEvent(model.Event{
			Type:    "graph.edge_removed",
			EdgeID:  id,
			Summary: "edge removed",
			Source:  "native",
		})
	}
	return existed
}

func (s *Store) Lineage(rootID string) model.Lineage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	includeAll := rootID == ""
	sessions := make([]model.Session, 0)
	for _, session := range s.sessions {
		if includeAll || session.ID == rootID || session.RootSessionID == rootID || session.ParentSessionID == rootID {
			sessions = append(sessions, session)
		}
	}
	sessionSet := make(map[string]bool, len(sessions))
	for _, session := range sessions {
		sessionSet[session.ID] = true
	}
	edges := make([]model.Edge, 0)
	for _, edge := range s.edges {
		if includeAll || sessionSet[edge.From] || sessionSet[edge.To] {
			edges = append(edges, edge)
		}
	}
	workspaces := make([]model.Workspace, 0)
	workspaceSet := map[string]bool{}
	for _, session := range sessions {
		for _, id := range session.WorkspaceIDs {
			workspaceSet[id] = true
		}
	}
	for id, workspace := range s.workspaces {
		if workspaceSet[id] {
			workspaces = append(workspaces, workspace)
		}
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartedAt.Before(sessions[j].StartedAt)
	})
	return model.Lineage{
		RootSessionID: rootID,
		Sessions:      sessions,
		Workspaces:    workspaces,
		Edges:         edges,
	}
}

func (s *Store) UpsertSource(source model.Source) model.Source {
	now := time.Now().UTC()
	if source.ID == "" {
		return source
	}
	if source.ObservedAt.IsZero() {
		source.ObservedAt = now
	}
	s.mu.Lock()
	s.sources[source.ID] = source
	s.mu.Unlock()
	return source
}

func (s *Store) GetSource(id string) (model.Source, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	source, ok := s.sources[id]
	if !ok {
		return model.Source{}, ErrNotFound
	}
	return source, nil
}

func (s *Store) ListSources() []model.Source {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.Source, 0, len(s.sources))
	for _, source := range s.sources {
		out = append(out, source)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Store) AppendEvent(event model.Event) model.Event {
	return s.appendEvent(event)
}

func (s *Store) ListEvents(since int64, limit int) []model.Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.Event, 0)
	for _, event := range s.events {
		if event.Seq > since {
			out = append(out, event)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}

func (s *Store) Subscribe(since int64) (int, <-chan model.Event, func()) {
	ch := make(chan model.Event, 128)
	s.mu.Lock()
	id := s.nextSubID
	s.nextSubID++
	s.subscribers[id] = ch
	backlog := make([]model.Event, 0)
	for _, event := range s.events {
		if event.Seq > since {
			backlog = append(backlog, event)
		}
	}
	s.mu.Unlock()

	go func() {
		for _, event := range backlog {
			ch <- event
		}
	}()

	cancel := func() {
		s.mu.Lock()
		if sub, ok := s.subscribers[id]; ok {
			delete(s.subscribers, id)
			close(sub)
		}
		s.mu.Unlock()
	}
	return id, ch, cancel
}

func (s *Store) appendEvent(event model.Event) model.Event {
	now := time.Now().UTC()
	if event.ObservedAt.IsZero() {
		event.ObservedAt = now
	}
	s.mu.Lock()
	s.nextSeq++
	event.Seq = s.nextSeq
	s.events = append(s.events, event)
	if len(s.events) > s.maxEvents {
		s.events = s.events[len(s.events)-s.maxEvents:]
	}
	subscribers := make([]chan model.Event, 0, len(s.subscribers))
	for _, ch := range s.subscribers {
		subscribers = append(subscribers, ch)
	}
	s.mu.Unlock()

	for _, ch := range subscribers {
		select {
		case ch <- event:
		default:
		}
	}
	return event
}

func deriveAttention(status model.Status) model.Attention {
	switch status {
	case model.StatusBlocked:
		return model.AttentionInput
	case model.StatusReviewable:
		return model.AttentionReview
	case model.StatusFailed, model.StatusDisconnected:
		return model.AttentionError
	default:
		return model.AttentionNone
	}
}

func isTerminal(status model.Status) bool {
	switch status {
	case model.StatusCompleted, model.StatusFailed, model.StatusStopped:
		return true
	default:
		return false
	}
}

func attentionRank(session model.Session) int {
	switch session.Attention {
	case model.AttentionInput, model.AttentionError:
		return 0
	case model.AttentionReview:
		return 1
	}
	switch session.Status {
	case model.StatusWorking:
		return 2
	case model.StatusIdle:
		return 3
	default:
		return 4
	}
}

func sessionMatchesWorkspace(session model.Session, query string) bool {
	if strings.Contains(session.CWD, query) {
		return true
	}
	for _, root := range session.WorkspaceRoots {
		if strings.Contains(root, query) {
			return true
		}
	}
	for _, id := range session.WorkspaceIDs {
		if strings.Contains(id, query) {
			return true
		}
	}
	return false
}
