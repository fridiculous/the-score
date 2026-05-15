package store

import (
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
	events := st.ListEvents(0, 0)
	if len(events) != 1 || events[0].Type != "session.started" {
		t.Fatalf("events = %#v", events)
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
