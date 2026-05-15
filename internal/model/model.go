package model

import "time"

type Status string

const (
	StatusIdle         Status = "idle"
	StatusWorking      Status = "working"
	StatusBlocked      Status = "blocked"
	StatusReviewable   Status = "reviewable"
	StatusCompleted    Status = "completed"
	StatusFailed       Status = "failed"
	StatusStopped      Status = "stopped"
	StatusDisconnected Status = "disconnected"
	StatusUnknown      Status = "unknown"
)

type Confidence string

const (
	ConfidenceHigh    Confidence = "high"
	ConfidenceMedium  Confidence = "medium"
	ConfidenceLow     Confidence = "low"
	ConfidenceUnknown Confidence = "unknown"
)

type Attention string

const (
	AttentionNone   Attention = "none"
	AttentionInput  Attention = "input"
	AttentionReview Attention = "review"
	AttentionError  Attention = "error"
)

type Agent struct {
	Name    string `json:"name,omitempty"`
	Vendor  string `json:"vendor,omitempty"`
	Adapter string `json:"adapter,omitempty"`
}

type Capabilities struct {
	Logs       bool `json:"logs"`
	Children   bool `json:"children"`
	Attach     bool `json:"attach"`
	Stop       bool `json:"stop"`
	SendPrompt bool `json:"sendPrompt"`
	Approvals  bool `json:"approvals"`
}

type Session struct {
	ID              string                 `json:"id"`
	Kind            string                 `json:"kind"`
	Agent           Agent                  `json:"agent,omitempty"`
	Title           string                 `json:"title,omitempty"`
	Summary         string                 `json:"summary,omitempty"`
	Status          Status                 `json:"status"`
	Attention       Attention              `json:"attention"`
	StatusDetail    string                 `json:"statusDetail,omitempty"`
	Confidence      Confidence             `json:"confidence"`
	Source          string                 `json:"source"`
	CWD             string                 `json:"cwd,omitempty"`
	WorkspaceRoots  []string               `json:"workspaceRoots,omitempty"`
	StartedAt       time.Time              `json:"startedAt,omitempty"`
	LastActivityAt  time.Time              `json:"lastActivityAt"`
	EndedAt         *time.Time             `json:"endedAt,omitempty"`
	ParentSessionID string                 `json:"parentSessionId,omitempty"`
	RootSessionID   string                 `json:"rootSessionId,omitempty"`
	RuntimeIDs      []string               `json:"runtimeIds,omitempty"`
	WorkspaceIDs    []string               `json:"workspaceIds,omitempty"`
	ArtifactIDs     []string               `json:"artifactIds,omitempty"`
	Capabilities    Capabilities           `json:"capabilities"`
	Meta            map[string]interface{} `json:"meta,omitempty"`
}

type Process struct {
	ID         string                 `json:"id"`
	Kind       string                 `json:"kind"`
	PID        int                    `json:"pid,omitempty"`
	PPID       int                    `json:"ppid,omitempty"`
	Command    string                 `json:"command,omitempty"`
	Args       string                 `json:"args,omitempty"`
	CWD        string                 `json:"cwd,omitempty"`
	SessionIDs []string               `json:"sessionIds,omitempty"`
	Liveness   string                 `json:"liveness"`
	Source     string                 `json:"source"`
	Confidence Confidence             `json:"confidence"`
	ObservedAt time.Time              `json:"observedAt"`
	Meta       map[string]interface{} `json:"meta,omitempty"`
}

type Workspace struct {
	ID         string                 `json:"id"`
	Kind       string                 `json:"kind"`
	Path       string                 `json:"path"`
	RepoRoot   string                 `json:"repoRoot,omitempty"`
	Branch     string                 `json:"branch,omitempty"`
	SessionIDs []string               `json:"sessionIds,omitempty"`
	Source     string                 `json:"source"`
	Confidence Confidence             `json:"confidence"`
	ObservedAt time.Time              `json:"observedAt"`
	Meta       map[string]interface{} `json:"meta,omitempty"`
}

type Edge struct {
	ID         string     `json:"id"`
	From       string     `json:"from"`
	To         string     `json:"to"`
	Type       string     `json:"type"`
	Source     string     `json:"source"`
	Confidence Confidence `json:"confidence"`
	ObservedAt time.Time  `json:"observedAt"`
}

type Event struct {
	Seq         int64                  `json:"seq"`
	Type        string                 `json:"type"`
	SessionID   string                 `json:"sessionId,omitempty"`
	ProcessID   string                 `json:"processId,omitempty"`
	WorkspaceID string                 `json:"workspaceId,omitempty"`
	EdgeID      string                 `json:"edgeId,omitempty"`
	Summary     string                 `json:"summary,omitempty"`
	Source      string                 `json:"source,omitempty"`
	ObservedAt  time.Time              `json:"observedAt"`
	Data        map[string]interface{} `json:"data,omitempty"`
}

type Source struct {
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	Kind            string                 `json:"kind"`
	Enabled         bool                   `json:"enabled"`
	Status          string                 `json:"status"`
	SupportLevel    string                 `json:"supportLevel"`
	DetectedVersion string                 `json:"detectedAgentVersion,omitempty"`
	DetectedPath    string                 `json:"detectedAgentPath,omitempty"`
	Capabilities    []string               `json:"capabilities,omitempty"`
	Diagnostics     []string               `json:"diagnostics,omitempty"`
	ObservedAt      time.Time              `json:"observedAt"`
	Meta            map[string]interface{} `json:"meta,omitempty"`
}

type Lineage struct {
	RootSessionID string      `json:"rootSessionId,omitempty"`
	Sessions      []Session   `json:"sessions"`
	Processes     []Process   `json:"processes,omitempty"`
	Workspaces    []Workspace `json:"workspaces,omitempty"`
	Edges         []Edge      `json:"edges"`
}
