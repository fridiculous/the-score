package runtime

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fridiculous/the-score/internal/model"
)

func TestInferAgentProcessSessionFromCommand(t *testing.T) {
	observedAt := time.Date(2026, 5, 15, 18, 0, 0, 0, time.UTC)
	session, ok := InferAgentProcessSession(model.Process{
		ID:      "process:42",
		PID:     42,
		PPID:    1,
		Command: "/opt/homebrew/bin/codex",
		Args:    "codex --model gpt-5",
		CWD:     "/repo",
	}, observedAt)
	if !ok {
		t.Fatal("expected codex process to infer a session")
	}
	if session.ID != "codex:process:42" {
		t.Fatalf("session ID = %q", session.ID)
	}
	if session.Source != "codex" {
		t.Fatalf("source = %q", session.Source)
	}
	if session.Status != model.StatusUnknown {
		t.Fatalf("status = %q", session.Status)
	}
	if session.Confidence != model.ConfidenceLow {
		t.Fatalf("confidence = %q", session.Confidence)
	}
	if len(session.WorkspaceIDs) != 1 || session.WorkspaceIDs[0] != "workspace:/repo" {
		t.Fatalf("workspace IDs = %#v", session.WorkspaceIDs)
	}
}

func TestInferAgentProcessSessionFromArgv0(t *testing.T) {
	_, ok := InferAgentProcessSession(model.Process{
		ID:      "process:42",
		PID:     42,
		Command: "python",
		Args:    "/usr/local/bin/claude --dangerously-skip-permissions",
	}, time.Now())
	if !ok {
		t.Fatal("expected argv0 path to infer a session")
	}
}

func TestInferAgentProcessSessionFromLauncherScript(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "hermes")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0600); err != nil {
		t.Fatal(err)
	}
	session, ok := InferAgentProcessSession(model.Process{
		ID:      "process:42",
		PID:     42,
		Command: "/Users/fridiculo",
		Args:    "/Users/fridiculous/projects/hermes-agent/.venv/bin/python3 " + bin,
	}, time.Now())
	if !ok {
		t.Fatal("expected launcher script to infer a session")
	}
	if session.Source != "hermes" {
		t.Fatalf("source = %q", session.Source)
	}
}

func TestInferAgentProcessSessionFromPathArgv0WhenCommandIsTruncated(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0600); err != nil {
		t.Fatal(err)
	}
	session, ok := InferAgentProcessSession(model.Process{
		ID:      "process:42",
		PID:     42,
		Command: "/opt/homebrew/li",
		Args:    bin,
	}, time.Now())
	if !ok {
		t.Fatal("expected argv0 path to infer a session when command is truncated")
	}
	if session.Source != "codex" {
		t.Fatalf("source = %q", session.Source)
	}
}

func TestInferAgentProcessSessionIgnoresMentionedAgentNames(t *testing.T) {
	cases := []model.Process{
		{ID: "process:1", PID: 1, Command: "rg", Args: "codex"},
		{ID: "process:2", PID: 2, Command: "score", Args: "run --source codex -- codex"},
		{ID: "process:3", PID: 3, Command: "zsh", Args: "-lc codex"},
		{ID: "process:4", PID: 4, Command: "/Applications/Co", Args: "/Applications/Codex.app/Contents/MacOS/Codex"},
		{ID: "process:5", PID: 5, Command: "python3", Args: "python3 sync.py --provider hermes"},
	}
	for _, tc := range cases {
		if session, ok := InferAgentProcessSession(tc, time.Now()); ok {
			t.Fatalf("unexpected session inferred for %#v: %#v", tc, session)
		}
	}
}

func TestSourcePackProcessFixtures(t *testing.T) {
	report, err := RunSourceFixtureTests("")
	if err != nil {
		t.Fatal(err)
	}
	if report.Total == 0 {
		t.Fatal("expected fixture cases")
	}
	if report.Failed != 0 {
		t.Fatalf("fixture failures = %#v", report.Results)
	}
}
