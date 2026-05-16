package api

import (
	"encoding/json"
	"testing"

	"github.com/fridiculous/the-score/internal/model"
	"github.com/fridiculous/the-score/internal/store"
)

type emptyProcessLister struct{}

func (emptyProcessLister) ListProcesses() ([]model.Process, error) {
	return []model.Process{}, nil
}

type staticProcessLister struct {
	processes []model.Process
}

func (l staticProcessLister) ListProcesses() ([]model.Process, error) {
	return l.processes, nil
}

func TestObservationUpsertAndSessionList(t *testing.T) {
	st := store.New()
	h := NewHandler(st, emptyProcessLister{})
	params, _ := json.Marshal(model.Session{
		ID:           "claude:1",
		Status:       model.StatusWorking,
		StatusDetail: "running tests",
		Source:       "claude",
	})
	if _, err := h.Handle("observations/upsertSession", params); err != nil {
		t.Fatalf("upsert error = %#v", err)
	}
	raw, err := h.Handle("sessions/list", nil)
	if err != nil {
		t.Fatalf("list error = %#v", err)
	}
	sessions, ok := raw.([]model.Session)
	if !ok {
		t.Fatalf("result type = %T", raw)
	}
	if len(sessions) != 1 {
		t.Fatalf("session count = %d", len(sessions))
	}
	if sessions[0].StatusDetail != "running tests" {
		t.Fatalf("status detail = %q", sessions[0].StatusDetail)
	}
}

func TestSessionsListInfersAgentProcesses(t *testing.T) {
	st := store.New()
	h := NewHandler(st, staticProcessLister{processes: []model.Process{
		{
			ID:      "process:100",
			Kind:    "process",
			PID:     100,
			PPID:    1,
			Command: "codex",
			Args:    "codex",
			CWD:     "/repo",
		},
	}})

	raw, err := h.Handle("sessions/list", nil)
	if err != nil {
		t.Fatalf("list error = %#v", err)
	}
	sessions, ok := raw.([]model.Session)
	if !ok {
		t.Fatalf("result type = %T", raw)
	}
	if len(sessions) != 1 {
		t.Fatalf("session count = %d", len(sessions))
	}
	if sessions[0].ID != "codex:process:100" {
		t.Fatalf("session ID = %q", sessions[0].ID)
	}
	if sessions[0].Source != "codex" {
		t.Fatalf("source = %q", sessions[0].Source)
	}
	if sessions[0].CWD != "/repo" {
		t.Fatalf("cwd = %q", sessions[0].CWD)
	}
}

func TestSessionsListInfersProcessLineage(t *testing.T) {
	st := store.New()
	h := NewHandler(st, staticProcessLister{processes: []model.Process{
		{ID: "process:100", Kind: "process", PID: 100, PPID: 1, Command: "codex", Args: "codex", CWD: "/repo"},
		{ID: "process:101", Kind: "process", PID: 101, PPID: 100, Command: "codex", Args: "codex", CWD: "/repo"},
	}})

	if _, err := h.Handle("sessions/list", nil); err != nil {
		t.Fatalf("list error = %#v", err)
	}
	raw, err := h.Handle("lineage/get", mustJSON(t, map[string]string{"sessionId": "codex:process:100"}))
	if err != nil {
		t.Fatalf("lineage error = %#v", err)
	}
	lineage, ok := raw.(model.Lineage)
	if !ok {
		t.Fatalf("result type = %T", raw)
	}
	if len(lineage.Sessions) != 2 {
		t.Fatalf("session count = %d", len(lineage.Sessions))
	}
	if len(lineage.Edges) != 1 {
		t.Fatalf("edge count = %d", len(lineage.Edges))
	}
	if lineage.Edges[0].From != "codex:process:101" || lineage.Edges[0].To != "codex:process:100" {
		t.Fatalf("edge = %#v", lineage.Edges[0])
	}
}

func mustJSON(t *testing.T, value interface{}) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
