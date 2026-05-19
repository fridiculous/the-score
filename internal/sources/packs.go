package sources

import (
	"time"

	"github.com/fridiculous/the-score/internal/model"
	"github.com/fridiculous/the-score/internal/version"
)

type Pack struct {
	ID              string
	Name            string
	Kind            string
	Status          string
	SupportLevel    string
	Commands        []string
	Capabilities    []string
	Lifecycle       model.SourceLifecycle
	Provenance      []string
	ConfidenceRules []string
	VersionProfiles []string
	Diagnostics     []string
}

func DefaultSources(observedAt time.Time) []model.Source {
	packs := []Pack{
		{
			ID:           "native",
			Name:         "Native Observation API",
			Kind:         "api",
			Status:       "active",
			SupportLevel: "native",
			Capabilities: []string{"sessions", "workspaces", "lineage", "events"},
			Lifecycle: model.SourceLifecycle{
				CanDetectStart:    true,
				CanDetectActivity: true,
				CanDetectWaiting:  true,
				CanDetectTerminal: true,
			},
			Provenance: []string{"client-reported observation API calls"},
		},
		{
			ID:           "process",
			Name:         "Process Table",
			Kind:         "runtime",
			Status:       "active",
			SupportLevel: "compatible",
			Capabilities: []string{"processes"},
			Lifecycle: model.SourceLifecycle{
				CanDetectLiveness: true,
			},
			Provenance: []string{"operating-system process table"},
			ConfidenceRules: []string{
				"process presence is baseline evidence only",
				"process records do not imply session status beyond liveness",
			},
		},
		{
			ID:           "git-worktree",
			Name:         "Git Worktree",
			Kind:         "workspace",
			Status:       "planned",
			SupportLevel: "not_installed",
			Capabilities: []string{"workspaces"},
			Provenance:   []string{"planned git metadata probes"},
			Diagnostics:  []string{"workspace discovery is declared but not implemented in this build"},
		},
		{
			ID:           "tmux",
			Name:         "tmux",
			Kind:         "runtime",
			Status:       "planned",
			SupportLevel: "not_installed",
			Capabilities: []string{"processes", "sessions"},
			Provenance:   []string{"planned tmux server queries"},
			Diagnostics:  []string{"tmux integration is declared but not implemented in this build"},
		},
		agentPack("claude", "Claude Code", []string{"claude"}),
		agentPack("codex", "Codex", []string{"codex"}),
		agentPack("opencode", "OpenCode", []string{"opencode"}),
		agentPack("hermes", "Hermes", []string{"hermes"}),
		agentPack("openclaw", "OpenClaw", []string{"openclaw"}),
		agentPack("nanoclaw", "NanoClaw", []string{"nanoclaw"}),
		{
			ID:           "mcp",
			Name:         "MCP",
			Kind:         "protocol",
			Status:       "planned",
			SupportLevel: "not_installed",
			Capabilities: []string{"tool_calls", "events"},
			Provenance:   []string{"planned MCP protocol events"},
			Diagnostics:  []string{"MCP source is part of the core bundle but not implemented in this build"},
		},
	}

	out := make([]model.Source, 0, len(packs))
	for _, pack := range packs {
		source := model.Source{
			ID:                pack.ID,
			Name:              pack.Name,
			Kind:              pack.Kind,
			Enabled:           true,
			Status:            pack.Status,
			SupportLevel:      pack.SupportLevel,
			SourcePackVersion: version.SourcePackVersion,
			Capabilities:      pack.Capabilities,
			Lifecycle:         pack.Lifecycle,
			Provenance:        pack.Provenance,
			ConfidenceRules:   pack.ConfidenceRules,
			VersionProfiles:   pack.VersionProfiles,
			Diagnostics:       pack.Diagnostics,
			ObservedAt:        observedAt,
		}
		if len(pack.Commands) > 0 {
			source.Meta = map[string]interface{}{
				"supportedCommands": pack.Commands,
			}
		}
		out = append(out, source)
	}
	return out
}

func agentPack(id, name string, commands []string) Pack {
	return Pack{
		ID:           id,
		Name:         name,
		Kind:         "session",
		Status:       "partial",
		SupportLevel: "process_probe",
		Commands:     commands,
		Capabilities: []string{"sessions", "processes"},
		Lifecycle: model.SourceLifecycle{
			CanDetectLiveness: true,
		},
		Provenance: []string{"operating-system process table"},
		ConfidenceRules: []string{
			"direct command or known launcher process infers a low-confidence session candidate",
			"shell mentions, app helpers, and arbitrary arguments must not infer a session",
			"structured source evidence can raise confidence in later versions",
		},
		VersionProfiles: []string{"unknown: degrade with diagnostics"},
		Diagnostics:     []string{"passive process detection is active; deeper session telemetry is not implemented in this build"},
	}
}
