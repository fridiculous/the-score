package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/fridiculous/the-score/internal/model"
)

func TestSessionUpsertDerivesDefaultsAndEvents(t *testing.T) {
	st := New()
	session := st.UpsertSession(model.Session{
		ID:     "codex:1",
		Status: model.StatusBlocked,
		Source: "codex",
	})
	if session.Kind != "session" {
		t.Fatalf("kind = %q", session.Kind)
	}
	if session.Attention != model.AttentionInput {
		t.Fatalf("attention = %q", session.Attention)
	}
	if session.RootSessionID != "codex:1" {
		t.Fatalf("root = %q", session.RootSessionID)
	}
	if session.LastSeenAt.IsZero() {
		t.Fatal("last seen was not defaulted")
	}
	if session.StatusUpdatedAt.IsZero() {
		t.Fatal("status updated timestamp was not defaulted")
	}
	if session.StatusSource != "codex" {
		t.Fatalf("status source = %q", session.StatusSource)
	}
	events := st.ListEvents(0, 0)
	if len(events) != 1 || events[0].Type != "session.started" {
		t.Fatalf("events = %#v", events)
	}
}

func TestSessionLifecycleSeparatesSeenFromActivity(t *testing.T) {
	st := New()
	startedAt := time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)
	seenAt := startedAt.Add(5 * time.Second)
	session := st.UpsertSession(model.Session{
		ID:             "codex:process:42",
		Status:         model.StatusUnknown,
		StatusDetail:   "process detected: codex",
		StatusSource:   "process",
		Source:         "codex",
		Confidence:     model.ConfidenceLow,
		StartedAt:      startedAt,
		LastSeenAt:     startedAt,
		LastActivityAt: startedAt,
		Meta: map[string]interface{}{
			"process": map[string]interface{}{"inferred": true},
		},
	})

	updated := st.UpsertSession(model.Session{
		ID:           session.ID,
		Status:       model.StatusUnknown,
		StatusDetail: session.StatusDetail,
		StatusSource: "process",
		Source:       session.Source,
		Confidence:   model.ConfidenceLow,
		LastSeenAt:   seenAt,
		Meta:         session.Meta,
	})

	if !updated.LastSeenAt.Equal(seenAt) {
		t.Fatalf("last seen = %s, want %s", updated.LastSeenAt, seenAt)
	}
	if !updated.LastActivityAt.Equal(startedAt) {
		t.Fatalf("last activity = %s, want %s", updated.LastActivityAt, startedAt)
	}
	events := st.ListEvents(0, 0)
	if len(events) != 1 || events[0].Type != "session.detected" {
		t.Fatalf("events = %#v", events)
	}
}

func TestSessionStatusChangeUsesLifecycleEvent(t *testing.T) {
	st := New()
	st.UpsertSession(model.Session{ID: "claude:1", Status: model.StatusWorking, Source: "claude"})
	st.UpsertSession(model.Session{ID: "claude:1", Status: model.StatusBlocked, Source: "claude"})

	events := st.ListEvents(0, 0)
	if len(events) != 2 {
		t.Fatalf("events = %#v", events)
	}
	if events[1].Type != "session.waiting_for_input" {
		t.Fatalf("event type = %q", events[1].Type)
	}
	if events[1].Data["previousStatus"] != model.StatusWorking {
		t.Fatalf("event data = %#v", events[1].Data)
	}
}

func TestRepeatedWorkspaceAndEdgeUpsertsDoNotSpamEvents(t *testing.T) {
	st := New()
	st.UpsertWorkspace(model.Workspace{ID: "workspace:/repo", Path: "/repo", Source: "process"})
	st.UpsertWorkspace(model.Workspace{ID: "workspace:/repo", Path: "/repo", Source: "process"})
	st.UpsertEdge(model.Edge{ID: "process:spawned_by:child->root", From: "child", To: "root", Type: "spawned_by", Source: "process"})
	st.UpsertEdge(model.Edge{ID: "process:spawned_by:child->root", From: "child", To: "root", Type: "spawned_by", Source: "process"})

	events := st.ListEvents(0, 0)
	if len(events) != 2 {
		t.Fatalf("events = %#v", events)
	}
	if events[0].Type != "workspace.discovered" || events[1].Type != "child.spawned" {
		t.Fatalf("events = %#v", events)
	}
}

func TestSQLitePersistsResourcesAndEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "score.db")
	st, err := NewSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	st.UpsertSession(model.Session{
		ID:             "codex:persisted",
		Status:         model.StatusWorking,
		Source:         "codex",
		LastActivityAt: now,
	})
	st.UpsertWorkspace(model.Workspace{
		ID:         "workspace:/repo",
		Path:       "/repo",
		Source:     "codex",
		ObservedAt: now,
	})
	st.UpsertEdge(model.Edge{
		From:       "codex:persisted",
		To:         "workspace:/repo",
		Type:       "uses_workspace",
		Source:     "codex",
		ObservedAt: now,
	})
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := NewSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if reopened.StoragePath() != path {
		t.Fatalf("storage path = %q", reopened.StoragePath())
	}
	if _, err := reopened.GetSession("codex:persisted"); err != nil {
		t.Fatalf("session not persisted: %v", err)
	}
	if _, err := reopened.GetWorkspace("workspace:/repo"); err != nil {
		t.Fatalf("workspace not persisted: %v", err)
	}
	if len(reopened.ListEdges()) != 1 {
		t.Fatalf("edge count = %d", len(reopened.ListEdges()))
	}
	if len(reopened.ListEvents(0, 0)) == 0 {
		t.Fatal("events were not persisted")
	}
}

func TestLineageIncludesHistoricalChildEdge(t *testing.T) {
	st := New()
	now := time.Now().UTC()
	st.UpsertSession(model.Session{ID: "root", Status: model.StatusWorking, LastActivityAt: now})
	st.UpsertSession(model.Session{ID: "child", Status: model.StatusCompleted, ParentSessionID: "root", RootSessionID: "root", LastActivityAt: now})
	st.UpsertEdge(model.Edge{From: "child", To: "root", Type: "spawned_by"})

	lineage := st.Lineage("root")
	if len(lineage.Sessions) != 2 {
		t.Fatalf("sessions = %d", len(lineage.Sessions))
	}
	if len(lineage.Edges) != 1 {
		t.Fatalf("edges = %d", len(lineage.Edges))
	}
}
