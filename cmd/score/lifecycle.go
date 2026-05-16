package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/fridiculous/the-score/internal/client"
	"github.com/fridiculous/the-score/internal/ipc"
)

func runStart(args []string) {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	scoredPath := fs.String("scored", "", "")
	_ = fs.Parse(args)

	if info, ok := daemonInfo(); ok {
		fmt.Printf("scored already running pid=%v socket=%s\n", info["pid"], ipc.DefaultAddress())
		return
	}

	path := *scoredPath
	if path == "" {
		var err error
		path, err = findScored()
		if err != nil {
			die(err)
		}
	}

	pidPath, logPath := lifecyclePaths()
	if err := os.MkdirAll(filepath.Dir(pidPath), 0700); err != nil {
		die(err)
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		die(err)
	}
	defer logFile.Close()

	cmd := exec.Command(path)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = os.Environ()
	detachCommand(cmd)
	if err := cmd.Start(); err != nil {
		die(err)
	}
	pid := cmd.Process.Pid
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(pid)+"\n"), 0600); err != nil {
		_ = cmd.Process.Kill()
		die(err)
	}
	_ = cmd.Process.Release()

	if err := waitForDaemon(true, 5*time.Second); err != nil {
		die(fmt.Errorf("scored started pid=%d but did not become reachable: %w; log=%s", pid, err, logPath))
	}
	fmt.Printf("scored started pid=%d socket=%s log=%s\n", pid, ipc.DefaultAddress(), logPath)
}

func runStop(args []string) {
	fs := flag.NewFlagSet("stop", flag.ExitOnError)
	_ = fs.Parse(args)

	usedAPI := false
	if c, _, err := client.Dial(); err == nil {
		var result map[string]bool
		if err := c.Call("daemon/shutdown", nil, &result); err == nil {
			usedAPI = true
		}
		_ = c.Close()
	}

	if !usedAPI {
		pid, err := readPid()
		if err != nil {
			fmt.Println("scored is not running")
			return
		}
		if err := signalProcess(pid); err != nil {
			die(err)
		}
	}

	if err := waitForDaemon(false, 5*time.Second); err != nil {
		pid, pidErr := readPid()
		if pidErr == nil {
			_ = killProcess(pid)
			_ = waitForDaemon(false, 2*time.Second)
		}
	}
	_ = os.Remove(pidPath())
	fmt.Println("scored stopped")
}

func runStatus(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "")
	_ = fs.Parse(args)

	info, ok := daemonInfo()
	if *asJSON {
		status := map[string]interface{}{
			"running": ok,
			"socket":  ipc.DefaultAddress(),
			"pidFile": pidPath(),
		}
		if ok {
			status["daemon"] = info
		}
		printJSON(status)
		return
	}
	if !ok {
		fmt.Printf("scored stopped socket=%s\n", ipc.DefaultAddress())
		return
	}
	fmt.Printf("scored running pid=%v socket=%s startedAt=%v\n", info["pid"], ipc.DefaultAddress(), info["startedAt"])
}

func daemonInfo() (map[string]interface{}, bool) {
	c, _, err := client.Dial()
	if err != nil {
		return nil, false
	}
	defer c.Close()
	var info map[string]interface{}
	if err := c.Call("daemon/info", nil, &info); err != nil {
		return nil, false
	}
	return info, true
}

func waitForDaemon(wantRunning bool, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		_, running := daemonInfo()
		if running == wantRunning {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	if wantRunning {
		return errors.New("daemon is not reachable")
	}
	return errors.New("daemon is still reachable")
}

func findScored() (string, error) {
	exe, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(exe), executableName("scored"))
		if isExecutable(candidate) {
			return candidate, nil
		}
	}
	if isExecutable(executableName("scored")) {
		return "./" + executableName("scored"), nil
	}
	if path, err := exec.LookPath("scored"); err == nil {
		return path, nil
	}
	return "", errors.New("cannot find scored; build it with: go build -o scored ./cmd/scored")
}

func executableName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode()&0111 != 0
}

func lifecyclePaths() (string, string) {
	if runtime.GOOS == "windows" {
		dir := os.TempDir()
		return filepath.Join(dir, "score-daemon.pid"), filepath.Join(dir, "score-daemon.log")
	}
	address := ipc.DefaultAddress()
	base := strings.TrimSuffix(address, filepath.Ext(address))
	return base + ".pid", base + ".log"
}

func pidPath() string {
	pidPath, _ := lifecyclePaths()
	return pidPath
}

func readPid() (int, error) {
	data, err := os.ReadFile(pidPath())
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func signalProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return proc.Kill()
	}
	return proc.Signal(os.Interrupt)
}

func killProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}
