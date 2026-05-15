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
