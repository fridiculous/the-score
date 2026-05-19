package api

import (
	"encoding/json"
	"testing"
	"time"

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

type countingProcessLister struct {
	count     int
	processes []model.Process
}

func (l *countingProcessLister) ListProcesses() ([]model.Process, error) {
	l.count++
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

func TestDaemonInfoReportsVersionContract(t *testing.T) {
	st := store.New()
	h := NewHandler(st, emptyProcessLister{})

	raw, err := h.Handle("daemon/info", nil)
	if err != nil {
		t.Fatalf("daemon info error = %#v", err)
	}
	info, ok := raw.(model.DaemonInfo)
	if !ok {
		t.Fatalf("result type = %T", raw)
	}
	if info.DaemonVersion == "" || info.APIVersion == "" || info.SourcePackVersion == "" {
		t.Fatalf("info missing version fields: %#v", info)
	}
}

func TestSourceFixtureAPI(t *testing.T) {
	st := store.New()
	h := NewHandler(st, emptyProcessLister{})

	raw, err := h.Handle("sources/testFixtures", nil)
	if err != nil {
		t.Fatalf("fixture error = %#v", err)
	}
	report, ok := raw.(model.SourceFixtureReport)
	if !ok {
		t.Fatalf("result type = %T", raw)
	}
	if report.Total == 0 || report.Failed != 0 {
		t.Fatalf("report = %#v", report)
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
	if sessions[0].Status != model.StatusUnknown {
		t.Fatalf("status = %q", sessions[0].Status)
	}
	if sessions[0].CWD != "/repo" {
		t.Fatalf("cwd = %q", sessions[0].CWD)
	}
}

func TestSessionsListThrottlesProcessReconcile(t *testing.T) {
	st := store.New()
	lister := &countingProcessLister{processes: []model.Process{
		{ID: "process:100", Kind: "process", PID: 100, PPID: 1, Command: "codex", Args: "codex", CWD: "/repo"},
	}}
	h := NewHandler(st, lister)

	if _, err := h.Handle("sessions/list", nil); err != nil {
		t.Fatalf("first list error = %#v", err)
	}
	if _, err := h.Handle("sessions/list", nil); err != nil {
		t.Fatalf("second list error = %#v", err)
	}
	if lister.count != 1 {
		t.Fatalf("process scans = %d, want 1", lister.count)
	}
	if _, err := h.Handle("sessions/list", mustJSON(t, map[string]bool{"forceRefresh": true})); err != nil {
		t.Fatalf("forced list error = %#v", err)
	}
	if lister.count != 2 {
		t.Fatalf("process scans after force refresh = %d, want 2", lister.count)
	}
}

func TestProcessReconcileUpdatesSeenWithoutActivity(t *testing.T) {
	st := store.New()
	lister := &countingProcessLister{processes: []model.Process{
		{ID: "process:100", Kind: "process", PID: 100, PPID: 1, Command: "codex", Args: "codex", CWD: "/repo"},
	}}
	h := NewHandler(st, lister)
	h.processReconcileEvery = 0

	raw, err := h.Handle("sessions/list", mustJSON(t, map[string]bool{"forceRefresh": true}))
	if err != nil {
		t.Fatalf("first list error = %#v", err)
	}
	first := raw.([]model.Session)[0]
	if first.LastSeenAt.IsZero() || first.LastActivityAt.IsZero() {
		t.Fatalf("missing lifecycle timestamps: %#v", first)
	}
	time.Sleep(time.Millisecond)
	raw, err = h.Handle("sessions/list", mustJSON(t, map[string]bool{"forceRefresh": true}))
	if err != nil {
		t.Fatalf("second list error = %#v", err)
	}
	second := raw.([]model.Session)[0]
	if !second.LastSeenAt.After(first.LastSeenAt) {
		t.Fatalf("last seen did not advance: %s <= %s", second.LastSeenAt, first.LastSeenAt)
	}
	if !second.LastActivityAt.Equal(first.LastActivityAt) {
		t.Fatalf("last activity changed: %s -> %s", first.LastActivityAt, second.LastActivityAt)
	}
	if !second.StatusUpdatedAt.Equal(first.StatusUpdatedAt) {
		t.Fatalf("status updated changed: %s -> %s", first.StatusUpdatedAt, second.StatusUpdatedAt)
	}
}

func TestProcessReconcileMarksMissingProcessDisconnected(t *testing.T) {
	st := store.New()
	lister := &countingProcessLister{processes: []model.Process{
		{ID: "process:100", Kind: "process", PID: 100, PPID: 1, Command: "codex", Args: "codex", CWD: "/repo"},
	}}
	h := NewHandler(st, lister)
	h.processReconcileEvery = 0
	h.processDisconnectAfter = 0

	if _, err := h.Handle("sessions/list", mustJSON(t, map[string]bool{"forceRefresh": true})); err != nil {
		t.Fatalf("first list error = %#v", err)
	}
	lister.processes = nil
	raw, err := h.Handle("sessions/list", mustJSON(t, map[string]bool{"forceRefresh": true}))
	if err != nil {
		t.Fatalf("second list error = %#v", err)
	}
	sessions := raw.([]model.Session)
	if len(sessions) != 1 {
		t.Fatalf("session count = %d", len(sessions))
	}
	if sessions[0].Status != model.StatusDisconnected {
		t.Fatalf("status = %q", sessions[0].Status)
	}
	if sessions[0].StatusSource != "process" {
		t.Fatalf("status source = %q", sessions[0].StatusSource)
	}
	if sessions[0].EndedAt == nil {
		t.Fatal("endedAt was not set")
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
