package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fridiculous/the-score/internal/api"
	"github.com/fridiculous/the-score/internal/client"
	"github.com/fridiculous/the-score/internal/model"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
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
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `score is the CLI client for scored.

Usage:
  score sessions [--all] [--status working,blocked] [--workspace PATH] [--source ID] [--json]
  score processes [--json]
  score workspaces [--json]
  score lineage [session-id] [--json]
  score events [--since N] [--watch] [--json]
  score history [--since N] [--json]
  score sources [doctor [source-id]] [--json]
  score inspect <id> [--json]
  score observe-session --id ID --status STATUS [--source ID] [--workspace PATH] [--activity TEXT]
`)
}

func runSessions(args []string) {
	fs := flag.NewFlagSet("sessions", flag.ExitOnError)
	all := fs.Bool("all", false, "")
	status := fs.String("status", "", "")
	workspace := fs.String("workspace", "", "")
	source := fs.String("source", "", "")
	asJSON := fs.Bool("json", false, "")
	_ = fs.Parse(args)

	var sessions []model.Session
	call("sessions/list", map[string]interface{}{
		"all":       *all,
		"status":    api.ParseStatusFilter(*status),
		"workspace": *workspace,
		"source":    *source,
	}, &sessions)
	if *asJSON {
		printJSON(sessions)
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSTATUS\tATTENTION\tSOURCE\tWORKSPACE\tACTIVITY\tUPDATED")
	for _, s := range sessions {
		updated := humanDuration(time.Since(s.LastActivityAt))
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", s.ID, s.Status, s.Attention, displaySource(s), displayWorkspace(s), firstNonEmpty(s.StatusDetail, s.Summary, s.Title), updated)
	}
	_ = w.Flush()
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
	}
	var raw interface{}
	call(method, params, &raw)
	if asJSON {
		printJSON(raw)
		return
	}
	data, _ := json.Marshal(raw)
	var sources []model.Source
	if err := json.Unmarshal(data, &sources); err == nil {
		printSources(sources)
		return
	}
	var source model.Source
	_ = json.Unmarshal(data, &source)
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
	fmt.Fprintf(os.Stderr, "score: cannot reach scored: %v\nstart it with: scored\n", err)
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
