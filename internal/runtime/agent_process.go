package runtime

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/fridiculous/the-score/internal/model"
)

var knownAgentCommands = map[string]string{
	"codex":    "codex",
	"claude":   "claude",
	"opencode": "opencode",
	"hermes":   "hermes",
	"openclaw": "openclaw",
	"nanoclaw": "nanoclaw",
}

var knownLauncherCommands = map[string]bool{
	"bash":    true,
	"bun":     true,
	"deno":    true,
	"node":    true,
	"npx":     true,
	"python":  true,
	"python3": true,
	"sh":      true,
	"zsh":     true,
}

func InferAgentProcessSession(proc model.Process, observedAt time.Time) (model.Session, bool) {
	source, ok := inferAgentSource(proc)
	if !ok {
		return model.Session{}, false
	}
	id := source + ":process:" + intString(proc.PID)
	title := processSessionTitle(source)
	return model.Session{
		ID:             id,
		Kind:           "session",
		Title:          title,
		Summary:        "detected running process",
		Status:         model.StatusWorking,
		StatusDetail:   "process active: " + title,
		Confidence:     model.ConfidenceLow,
		Source:         source,
		CWD:            proc.CWD,
		WorkspaceRoots: workspaceRoots(proc.CWD),
		StartedAt:      observedAt,
		LastActivityAt: observedAt,
		RootSessionID:  id,
		RuntimeIDs:     []string{proc.ID},
		WorkspaceIDs:   workspaceIDs(proc.CWD),
		Capabilities: model.Capabilities{
			Logs:     false,
			Children: false,
		},
		Meta: map[string]interface{}{
			"process": map[string]interface{}{
				"pid":        proc.PID,
				"ppid":       proc.PPID,
				"command":    proc.Command,
				"args":       proc.Args,
				"inferred":   true,
				"confidence": model.ConfidenceLow,
			},
		},
	}, true
}

func processSessionTitle(source string) string {
	if source == "" {
		return "agent"
	}
	return source
}

func inferAgentSource(proc model.Process) (string, bool) {
	if isAppBundleProcess(proc.Args) {
		return "", false
	}
	commandBase := normalizeCommandName(proc.Command)
	if source, ok := knownAgentCommands[commandBase]; ok {
		return source, true
	}

	argv0 := firstArg(proc.Args)
	argv0Base := normalizeCommandName(argv0)
	if argv0Base == "" {
		return "", false
	}
	source, ok := knownAgentCommands[argv0Base]
	if !ok {
		return "", false
	}
	if argv0Base == commandBase || knownLauncherCommands[commandBase] {
		return source, true
	}
	if looksLikeCommandPath(argv0) && commandPathExists(argv0) {
		return source, true
	}
	return "", false
}

func normalizeCommandName(command string) string {
	if command == "" {
		return ""
	}
	command = strings.Trim(command, `"'`)
	base := filepath.Base(command)
	if runtime.GOOS == "windows" {
		base = strings.TrimSuffix(base, ".exe")
	}
	return strings.ToLower(base)
}

func firstArg(args string) string {
	fields := strings.Fields(args)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func looksLikeCommandPath(command string) bool {
	return strings.Contains(command, "/") || strings.Contains(command, `\`)
}

func isAppBundleProcess(args string) bool {
	return strings.Contains(args, ".app/Contents/")
}

func commandPathExists(command string) bool {
	_, err := os.Stat(command)
	return err == nil
}

func workspaceRoots(cwd string) []string {
	if cwd == "" {
		return nil
	}
	return []string{cwd}
}

func workspaceIDs(cwd string) []string {
	if cwd == "" {
		return nil
	}
	return []string{"workspace:" + cwd}
}

func intString(value int) string {
	return strconv.Itoa(value)
}
