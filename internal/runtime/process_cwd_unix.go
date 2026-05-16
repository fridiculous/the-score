//go:build !windows

package runtime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func LookupProcessCWD(pid int) (string, bool) {
	if pid <= 0 {
		return "", false
	}
	if runtime.GOOS == "linux" {
		cwd, err := os.Readlink(fmt.Sprintf("/proc/%d/cwd", pid))
		return cwd, err == nil && cwd != ""
	}
	if runtime.GOOS == "darwin" {
		return lookupProcessCWDWithLsof(pid)
	}
	return "", false
}

func lookupProcessCWDWithLsof(pid int) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, lsofCommand(), "-a", "-p", strconv.Itoa(pid), "-d", "cwd", "-Fn").Output()
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "n") && len(line) > 1 {
			return strings.TrimSpace(line[1:]), true
		}
	}
	return "", false
}

func lsofCommand() string {
	if runtime.GOOS == "darwin" {
		if _, err := os.Stat("/usr/sbin/lsof"); err == nil {
			return "/usr/sbin/lsof"
		}
	}
	return "lsof"
}
