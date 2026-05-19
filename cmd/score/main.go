package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/fridiculous/the-score/internal/api"
	"github.com/fridiculous/the-score/internal/client"
	"github.com/fridiculous/the-score/internal/model"
	"github.com/fridiculous/the-score/internal/version"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "version":
		runVersion(os.Args[2:])
	case "start":
		runStart(os.Args[2:])
	case "stop":
		runStop(os.Args[2:])
	case "status":
		runStatus(os.Args[2:])
	case "sessions":
		runSessions(os.Args[2:])
	case "processes":
		runProcesses(os.Args[2:])
	case "workspaces":
		runWorkspaces(os.Args[2:])
	case "lineage":
		runLineage(os.Args[2:])
	case "events":
		runEvents(os.Args[2:])
	case "history":
		runHistory(os.Args[2:])
	case "sources":
		runSources(os.Args[2:])
	case "inspect":
		runInspect(os.Args[2:])
	case "observe-session":
		runObserveSession(os.Args[2:])
	case "run":
		runObservedCommand(os.Args[2:])
	default:
		if isSourceShortcut(os.Args[1]) {
			runSourceShortcut(os.Args[1], os.Args[2:])
			return
		}
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `score is the CLI client for scored.

Usage:
  score version [--daemon] [--json]
  score start [--scored PATH]
  score stop
  score status [--json]
  score sessions [--all] [--status working,blocked] [--workspace PATH] [--source ID] [--watch] [--interval DURATION] [--refresh] [--json]
  score processes [--json]
  score workspaces [--json]
  score lineage [session-id] [--json]
  score events [--since N] [--watch] [--json]
  score history [--since N] [--json]
  score sources [doctor [source-id] | test-fixtures [source-id]] [--json]
  score mcp [--json]
  score inspect <id> [--json]
  score observe-session --id ID --status STATUS [--source ID] [--workspace PATH] [--activity TEXT]
  score run [--id ID] [--source ID] [--workspace PATH] [--title TEXT] -- COMMAND [ARGS...]
`)
}

func runVersion(args []string) {
	fs := flag.NewFlagSet("version", flag.ExitOnError)
	daemon := fs.Bool("daemon", false, "")
	asJSON := fs.Bool("json", false, "")
	_ = fs.Parse(args)

	cliVersion := map[string]string{
		"client":      "score",
		"version":     version.Version,
		"apiVersion":  version.APIVersion,
		"buildCommit": version.BuildCommit,
	}
	if !*daemon {
		if *asJSON {
			printJSON(cliVersion)
			return
		}
		fmt.Printf("score %s api=%s commit=%s\n", version.Version, version.APIVersion, version.BuildCommit)
		return
	}

	var info model.DaemonInfo
	call("daemon/info", nil, &info)
	if *asJSON {
		printJSON(map[string]interface{}{
			"client": cliVersion,
			"daemon": info,
		})
		return
	}
	fmt.Printf("score %s api=%s commit=%s\n", version.Version, version.APIVersion, version.BuildCommit)
	fmt.Printf("scored %s api=%s sourcePacks=%s commit=%s storage=%s\n", info.DaemonVersion, info.APIVersion, info.SourcePackVersion, info.BuildCommit, firstNonEmpty(info.StoragePath, "-"))
}

func runSessions(args []string) {
	fs := flag.NewFlagSet("sessions", flag.ExitOnError)
	all := fs.Bool("all", false, "")
	status := fs.String("status", "", "")
	workspace := fs.String("workspace", "", "")
	source := fs.String("source", "", "")
	watch := fs.Bool("watch", false, "")
	interval := fs.Duration("interval", time.Second, "")
	refresh := fs.Bool("refresh", false, "")
	asJSON := fs.Bool("json", false, "")
	_ = fs.Parse(args)
	if *interval <= 0 {
		die(fmt.Errorf("--interval must be greater than zero"))
	}
	if *watch {
		watchSessions(*all, *status, *workspace, *source, *interval, *refresh, *asJSON)
		return
	}

	sessions := fetchSessions(*all, *status, *workspace, *source, *refresh, false)
	if *asJSON {
		printJSON(sessions)
		return
	}
	printSessions(sessions)
}

func watchSessions(all bool, status, workspace, source string, interval time.Duration, refresh bool, asJSON bool) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	order := newSessionOrder()
	tick := 0

	render := func() {
		sessions := order.apply(fetchSessions(all, status, workspace, source, refresh, true))
		if asJSON {
			printJSON(sessions)
			return
		}
		refreshStatus := fetchRefreshStatus()
		clearScreen()
		fmt.Printf("score sessions --watch  interval=%s  %s\n\n", interval, formatRefreshStatus(refreshStatus, tick))
		printSessions(sessions)
		tick++
	}

	render()
	for {
		select {
		case <-ticker.C:
			render()
		case <-signals:
			return
		}
	}
}

func fetchSessions(all bool, status, workspace, source string, forceRefresh bool, asyncRefresh bool) []model.Session {
	var sessions []model.Session
	call("sessions/list", map[string]interface{}{
		"all":          all,
		"status":       api.ParseStatusFilter(status),
		"workspace":    workspace,
		"source":       source,
		"forceRefresh": forceRefresh,
		"asyncRefresh": asyncRefresh,
	}, &sessions)
	return sessions
}

func fetchRefreshStatus() model.RefreshStatus {
	var status model.RefreshStatus
	if err := callErr("refresh/status", nil, &status); err != nil {
		return model.RefreshStatus{}
	}
	return status
}

type sessionOrder struct {
	positions map[string]int
	next      int
}

func newSessionOrder() *sessionOrder {
	return &sessionOrder{positions: make(map[string]int)}
}

func (o *sessionOrder) apply(sessions []model.Session) []model.Session {
	for _, session := range sessions {
		if _, ok := o.positions[session.ID]; ok {
			continue
		}
		o.positions[session.ID] = o.next
		o.next++
	}
	out := append([]model.Session(nil), sessions...)
	sort.SliceStable(out, func(i, j int) bool {
		return o.positions[out[i].ID] < o.positions[out[j].ID]
	})
	return out
}

func formatRefreshStatus(status model.RefreshStatus, tick int) string {
	processes := status.Processes
	if processes.Running {
		frames := []string{"-", "\\", "|", "/"}
		return "refresh=processes:" + frames[tick%len(frames)]
	}
	if processes.LastError != "" {
		return "refresh=processes:error"
	}
	if !processes.LastFinishedAt.IsZero() {
		return fmt.Sprintf("refresh=processes:idle last=%s duration=%dms", humanDuration(time.Since(processes.LastFinishedAt)), processes.LastDurationMillis)
	}
	return "refresh=processes:waiting"
}

func printSessions(sessions []model.Session) {
	if len(sessions) == 0 {
		fmt.Println("No active sessions.")
		fmt.Println("Create one with: score run -- <command>")
		fmt.Println("Or report one with: score observe-session --id ID --status working")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSTATUS\tATTENTION\tSOURCE\tWORKSPACE\tACTIVITY\tSEEN")
	for _, s := range sessions {
		seen := humanDuration(time.Since(sessionSeenAt(s)))
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", s.ID, s.Status, s.Attention, displaySource(s), displayWorkspace(s), firstNonEmpty(s.StatusDetail, s.Summary, s.Title), seen)
	}
	_ = w.Flush()
}

func sessionSeenAt(session model.Session) time.Time {
	if !session.LastSeenAt.IsZero() {
		return session.LastSeenAt
	}
	if !session.LastActivityAt.IsZero() {
		return session.LastActivityAt
	}
	if !session.StartedAt.IsZero() {
		return session.StartedAt
	}
	return time.Now().UTC()
}

func clearScreen() {
	if os.Getenv("TERM") == "dumb" {
		return
	}
	fmt.Print("\033[H\033[2J")
}

func runProcesses(args []string) {
	fs := flag.NewFlagSet("processes", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "")
	_ = fs.Parse(args)
	var processes []model.Process
	call("processes/list", nil, &processes)
	if *asJSON {
		printJSON(processes)
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tPID\tPPID\tCOMMAND\tLIVENESS\tSOURCE")
	for _, p := range processes {
		fmt.Fprintf(w, "%s\t%d\t%d\t%s\t%s\t%s\n", p.ID, p.PID, p.PPID, p.Command, p.Liveness, p.Source)
	}
	_ = w.Flush()
}

func runWorkspaces(args []string) {
	fs := flag.NewFlagSet("workspaces", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "")
	_ = fs.Parse(args)
	var workspaces []model.Workspace
	call("workspaces/list", nil, &workspaces)
	if *asJSON {
		printJSON(workspaces)
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tKIND\tPATH\tBRANCH\tSOURCE")
	for _, ws := range workspaces {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", ws.ID, ws.Kind, ws.Path, ws.Branch, ws.Source)
	}
	_ = w.Flush()
}

func runLineage(args []string) {
	fs := flag.NewFlagSet("lineage", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "")
	_ = fs.Parse(args)
	sessionID := ""
	if fs.NArg() > 0 {
		sessionID = fs.Arg(0)
	}
	var lineage model.Lineage
	call("lineage/get", map[string]string{"sessionId": sessionID}, &lineage)
	if *asJSON {
		printJSON(lineage)
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if len(lineage.Sessions) == 0 && len(lineage.Edges) == 0 {
		if sessionID == "" {
			fmt.Fprintln(w, "No lineage yet.")
			fmt.Fprintln(w, "Create linked sessions with: score run --parent <session-id> -- <command>")
		} else {
			fmt.Fprintf(w, "No lineage found for %s.\n", sessionID)
		}
		_ = w.Flush()
		return
	}
	fmt.Fprintln(w, "SESSION\tSTATUS\tPARENT\tROOT\tACTIVITY")
	for _, s := range lineage.Sessions {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", s.ID, s.Status, s.ParentSessionID, s.RootSessionID, firstNonEmpty(s.StatusDetail, s.Summary, s.Title))
	}
	if len(lineage.Edges) > 0 {
		fmt.Fprintln(w, "\nEDGE\tTYPE\tFROM\tTO")
		for _, e := range lineage.Edges {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", e.ID, e.Type, e.From, e.To)
		}
	}
	_ = w.Flush()
}

func runEvents(args []string) {
	fs := flag.NewFlagSet("events", flag.ExitOnError)
	since := fs.Int64("since", 0, "")
	watch := fs.Bool("watch", false, "")
	asJSON := fs.Bool("json", false, "")
	_ = fs.Parse(args)
	if *watch {
		c, _, err := client.Dial()
		if err != nil {
			dieDaemon(err)
		}
		defer c.Close()
		events, err := c.SubscribeEvents(*since)
		if err != nil {
			die(err)
		}
		for event := range events {
			if *asJSON {
				printJSON(event)
			} else {
				fmt.Printf("%d\t%s\t%s\t%s\n", event.Seq, event.Type, event.SessionID, event.Summary)
			}
		}
		return
	}
	var events []model.Event
	call("events/list", map[string]interface{}{"since": *since, "limit": 0}, &events)
	if *asJSON {
		printJSON(events)
		return
	}
	printEvents(events)
}

func runHistory(args []string) {
	fs := flag.NewFlagSet("history", flag.ExitOnError)
	since := fs.Int64("since", 0, "")
	asJSON := fs.Bool("json", false, "")
	_ = fs.Parse(args)
	var events []model.Event
	call("events/list", map[string]interface{}{"since": *since, "limit": 100}, &events)
	if *asJSON {
		printJSON(events)
		return
	}
	printEvents(events)
}

func runSources(args []string) {
	asJSON := false
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--json" {
			asJSON = true
			continue
		}
		filtered = append(filtered, arg)
	}
	args = filtered
	method := "sources/list"
	params := interface{}(nil)
	if len(args) > 0 && args[0] == "doctor" {
		method = "sources/doctor"
		if len(args) > 1 {
			params = map[string]string{"id": args[1]}
		}
	} else if len(args) > 0 && args[0] == "test-fixtures" {
		method = "sources/testFixtures"
		if len(args) > 1 {
			params = map[string]string{"id": args[1]}
		}
	}
	var raw interface{}
	call(method, params, &raw)
	if asJSON {
		printJSON(raw)
		return
	}
	data, _ := json.Marshal(raw)
	var report model.SourceFixtureReport
	if err := json.Unmarshal(data, &report); err == nil && report.SourcePackVersion != "" {
		printSourceFixtureReport(report)
		return
	}
	var sources []model.Source
	if err := json.Unmarshal(data, &sources); err == nil {
		printSources(sources)
		return
	}
	var source model.Source
	_ = json.Unmarshal(data, &source)
	printSources([]model.Source{source})
}

func isSourceShortcut(command string) bool {
	switch command {
	case "native", "process", "git-worktree", "tmux", "claude", "codex", "opencode", "hermes", "openclaw", "nanoclaw", "mcp":
		return true
	default:
		return false
	}
}

func runSourceShortcut(id string, args []string) {
	fs := flag.NewFlagSet(id, flag.ExitOnError)
	asJSON := fs.Bool("json", false, "")
	_ = fs.Parse(args)
	if fs.NArg() != 0 {
		usage()
		os.Exit(2)
	}
	var source model.Source
	call("sources/doctor", map[string]string{"id": id}, &source)
	if *asJSON {
		printJSON(source)
		return
	}
	printSources([]model.Source{source})
}

func runInspect(args []string) {
	fs := flag.NewFlagSet("inspect", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "")
	_ = fs.Parse(args)
	if fs.NArg() != 1 {
		usage()
		os.Exit(2)
	}
	id := fs.Arg(0)
	var session model.Session
	if err := callErr("sessions/get", map[string]string{"id": id}, &session); err == nil {
		printMaybeJSON(session, *asJSON)
		return
	}
	var workspace model.Workspace
	if err := callErr("workspaces/get", map[string]string{"id": id}, &workspace); err == nil {
		printMaybeJSON(workspace, *asJSON)
		return
	}
	var process model.Process
	if err := callErr("processes/get", map[string]string{"id": id}, &process); err == nil {
		printMaybeJSON(process, *asJSON)
		return
	}
	die(fmt.Errorf("not found: %s", id))
}

func runObserveSession(args []string) {
	fs := flag.NewFlagSet("observe-session", flag.ExitOnError)
	id := fs.String("id", "", "")
	status := fs.String("status", "working", "")
	source := fs.String("source", "native", "")
	workspace := fs.String("workspace", "", "")
	activity := fs.String("activity", "", "")
	title := fs.String("title", "", "")
	parent := fs.String("parent", "", "")
	root := fs.String("root", "", "")
	asJSON := fs.Bool("json", false, "")
	_ = fs.Parse(args)
	if *id == "" {
		die(fmt.Errorf("--id is required"))
	}
	workspaceIDs := []string(nil)
	if *workspace != "" {
		workspaceIDs = []string{"workspace:" + *workspace}
	}
	session := model.Session{
		ID:              *id,
		Status:          model.Status(*status),
		StatusDetail:    *activity,
		StatusSource:    *source,
		Title:           *title,
		Source:          *source,
		CWD:             *workspace,
		ParentSessionID: *parent,
		RootSessionID:   *root,
		WorkspaceIDs:    workspaceIDs,
		Confidence:      model.ConfidenceHigh,
		LastActivityAt:  time.Now().UTC(),
	}
	var result model.Session
	call("observations/upsertSession", session, &result)
	if *workspace != "" {
		workspaceID := "workspace:" + *workspace
		var ws model.Workspace
		call("observations/upsertWorkspace", model.Workspace{
			ID:         workspaceID,
			Kind:       "workspace",
			Path:       *workspace,
			SessionIDs: []string{*id},
			Source:     *source,
			Confidence: model.ConfidenceHigh,
		}, &ws)
	}
	if *parent != "" {
		var edge model.Edge
		call("observations/upsertEdge", model.Edge{
			From:       *id,
			To:         *parent,
			Type:       "spawned_by",
			Source:     *source,
			Confidence: model.ConfidenceHigh,
		}, &edge)
	}
	if *asJSON {
		printJSON(result)
		return
	}
	fmt.Println(result.ID)
}

func runObservedCommand(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	id := fs.String("id", "", "")
	source := fs.String("source", "", "")
	workspace := fs.String("workspace", "", "")
	title := fs.String("title", "", "")
	parent := fs.String("parent", "", "")
	root := fs.String("root", "", "")
	_ = fs.Parse(args)

	commandArgs := fs.Args()
	if len(commandArgs) == 0 {
		die(fmt.Errorf("COMMAND is required"))
	}
	if *workspace == "" {
		wd, err := os.Getwd()
		if err != nil {
			die(err)
		}
		*workspace = wd
	}
	if *source == "" {
		*source = inferSource(commandArgs[0])
	}
	if *id == "" {
		*id = fmt.Sprintf("%s:%d", *source, time.Now().UnixNano())
	}
	if *title == "" {
		*title = filepath.Base(commandArgs[0])
	}
	if *parent == "" {
		*parent = os.Getenv("SCORE_SESSION_ID")
	}
	if *root == "" {
		*root = os.Getenv("SCORE_ROOT_SESSION_ID")
	}
	if *root == "" {
		if *parent != "" {
			*root = *parent
		} else {
			*root = *id
		}
	}

	var health map[string]string
	call("health/check", nil, &health)

	cmd := exec.Command(commandArgs[0], commandArgs[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = *workspace
	workspaceID := "workspace:" + *workspace
	cmd.Env = append(os.Environ(),
		"SCORE_SESSION_ID="+*id,
		"SCORE_PARENT_SESSION_ID="+*parent,
		"SCORE_ROOT_SESSION_ID="+*root,
		"SCORE_WORKSPACE_ID="+workspaceID,
		"SCORE_SOURCE="+*source,
	)

	startedAt := time.Now().UTC()
	if err := cmd.Start(); err != nil {
		endedAt := time.Now().UTC()
		upsertRunSession(*id, *source, *workspace, workspaceID, *title, *parent, *root, nil, model.StatusFailed, "start failed: "+err.Error(), startedAt, &endedAt, commandArgs)
		die(err)
	}

	runtimeID := "process:" + strconv.Itoa(cmd.Process.Pid)
	commandLabel := commandDisplay(commandArgs)
	upsertRunSession(*id, *source, *workspace, workspaceID, *title, *parent, *root, []string{runtimeID}, model.StatusWorking, "running: "+commandLabel, startedAt, nil, commandArgs)
	upsertRunWorkspace(workspaceID, *workspace, *id, *source)
	if *parent != "" {
		upsertRunEdge(*id, *parent, *source)
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	done := make(chan struct{})
	var stopped atomic.Bool
	go func() {
		select {
		case sig := <-signals:
			stopped.Store(true)
			_ = cmd.Process.Signal(sig)
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				_ = cmd.Process.Kill()
			}
		case <-done:
		}
	}()

	err := cmd.Wait()
	close(done)
	endedAt := time.Now().UTC()

	status := model.StatusCompleted
	detail := "completed: " + commandLabel
	exitCode := 0
	if stopped.Load() {
		status = model.StatusStopped
		detail = "stopped: " + commandLabel
		exitCode = 130
	} else if err != nil {
		status = model.StatusFailed
		detail = "failed: " + err.Error()
		exitCode = 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}
	upsertRunSession(*id, *source, *workspace, workspaceID, *title, *parent, *root, []string{runtimeID}, status, detail, startedAt, &endedAt, commandArgs)
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func inferSource(command string) string {
	base := strings.TrimSuffix(filepath.Base(command), ".exe")
	if base == "" || base == "." || base == string(filepath.Separator) {
		return "run"
	}
	return strings.ToLower(base)
}

func commandDisplay(commandArgs []string) string {
	if len(commandArgs) == 0 {
		return "command"
	}
	label := filepath.Base(commandArgs[0])
	if label == "" || label == "." || label == string(filepath.Separator) {
		label = "command"
	}
	argCount := len(commandArgs) - 1
	if argCount <= 0 {
		return label
	}
	if argCount == 1 {
		return label + " (+1 arg)"
	}
	return fmt.Sprintf("%s (+%d args)", label, argCount)
}

func upsertRunSession(id, source, workspace, workspaceID, title, parent, root string, runtimeIDs []string, status model.Status, detail string, startedAt time.Time, endedAt *time.Time, commandArgs []string) {
	var result model.Session
	command := ""
	if len(commandArgs) > 0 {
		command = filepath.Base(commandArgs[0])
	}
	argCount := len(commandArgs) - 1
	if argCount < 0 {
		argCount = 0
	}
	call("observations/upsertSession", model.Session{
		ID:              id,
		Status:          status,
		StatusDetail:    detail,
		StatusSource:    source,
		Title:           title,
		Source:          source,
		CWD:             workspace,
		ParentSessionID: parent,
		RootSessionID:   root,
		RuntimeIDs:      runtimeIDs,
		WorkspaceIDs:    []string{workspaceID},
		Confidence:      model.ConfidenceHigh,
		StartedAt:       startedAt,
		LastActivityAt:  time.Now().UTC(),
		EndedAt:         endedAt,
		Meta: map[string]interface{}{
			"run": map[string]interface{}{
				"command":  command,
				"argCount": argCount,
			},
		},
	}, &result)
}

func upsertRunWorkspace(workspaceID, workspace, sessionID, source string) {
	var ws model.Workspace
	call("observations/upsertWorkspace", model.Workspace{
		ID:         workspaceID,
		Kind:       "workspace",
		Path:       workspace,
		SessionIDs: []string{sessionID},
		Source:     source,
		Confidence: model.ConfidenceHigh,
	}, &ws)
}

func upsertRunEdge(id, parent, source string) {
	var edge model.Edge
	call("observations/upsertEdge", model.Edge{
		From:       id,
		To:         parent,
		Type:       "spawned_by",
		Source:     source,
		Confidence: model.ConfidenceHigh,
	}, &edge)
}

func call(method string, params interface{}, result interface{}) {
	if err := callErr(method, params, result); err != nil {
		die(err)
	}
}

func callErr(method string, params interface{}, result interface{}) error {
	c, _, err := client.Dial()
	if err != nil {
		dieDaemon(err)
	}
	defer c.Close()
	return c.Call(method, params, result)
}

func dieDaemon(err error) {
	fmt.Fprintf(os.Stderr, "score: cannot reach scored: %v\nstart it with: score start\n", err)
	os.Exit(1)
}

func die(err error) {
	fmt.Fprintf(os.Stderr, "score: %v\n", err)
	os.Exit(1)
}

func printJSON(value interface{}) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		die(err)
	}
	fmt.Println(string(data))
}

func printMaybeJSON(value interface{}, asJSON bool) {
	if asJSON {
		printJSON(value)
		return
	}
	printJSON(value)
}

func printEvents(events []model.Event) {
	if len(events) == 0 {
		fmt.Println("No events yet.")
		fmt.Println("Generate history with: score run -- <command>")
		fmt.Println("Or report metadata with: score observe-session --id ID --status working")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SEQ\tTYPE\tSESSION\tSOURCE\tSUMMARY\tOBSERVED")
	for _, event := range events {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\n", event.Seq, event.Type, event.SessionID, event.Source, event.Summary, event.ObservedAt.Format(time.RFC3339))
	}
	_ = w.Flush()
}

func printSources(sources []model.Source) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSTATUS\tSUPPORT\tKIND\tCAPABILITIES\tDIAGNOSTICS")
	for _, source := range sources {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", source.ID, source.Status, source.SupportLevel, source.Kind, strings.Join(source.Capabilities, ","), strings.Join(source.Diagnostics, "; "))
	}
	_ = w.Flush()
}

func printSourceFixtureReport(report model.SourceFixtureReport) {
	scope := "all"
	if report.FilterSourceID != "" {
		scope = report.FilterSourceID
	}
	fmt.Printf("source-pack fixtures scope=%s version=%s passed=%d failed=%d total=%d\n", scope, report.SourcePackVersion, report.Passed, report.Failed, report.Total)
	for _, result := range report.Results {
		status := "PASS"
		if !result.Passed {
			status = "FAIL"
		}
		if result.Diagnostic == "" {
			fmt.Printf("%s\t%s\t%s\n", status, result.SourceID, result.Name)
			continue
		}
		fmt.Printf("%s\t%s\t%s\t%s\n", status, result.SourceID, result.Name, result.Diagnostic)
	}
}

func displaySource(s model.Session) string {
	if s.Source != "" {
		return s.Source
	}
	if s.Agent.Adapter != "" {
		return s.Agent.Adapter
	}
	return s.Agent.Name
}

func displayWorkspace(s model.Session) string {
	if s.CWD != "" {
		return s.CWD
	}
	if len(s.WorkspaceRoots) > 0 {
		return s.WorkspaceRoots[0]
	}
	if len(s.WorkspaceIDs) > 0 {
		return s.WorkspaceIDs[0]
	}
	return "-"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return "-"
}

func humanDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}
