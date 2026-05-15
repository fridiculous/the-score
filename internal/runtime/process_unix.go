//go:build !windows

package runtime

import (
	"bytes"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/fridiculous/the-score/internal/model"
)

type SystemProcessLister struct{}

func (SystemProcessLister) ListProcesses() ([]model.Process, error) {
	out, err := exec.Command("ps", "-axo", "pid=,ppid=,comm=,args=").Output()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	lines := bytes.Split(out, []byte{'\n'})
	processes := make([]model.Process, 0, len(lines))
	for _, raw := range lines {
		line := strings.TrimSpace(string(raw))
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		ppid, _ := strconv.Atoi(fields[1])
		command := fields[2]
		args := ""
		if len(fields) > 3 {
			args = strings.Join(fields[3:], " ")
		}
		processes = append(processes, model.Process{
			ID:         "process:" + strconv.Itoa(pid),
			Kind:       "process",
			PID:        pid,
			PPID:       ppid,
			Command:    command,
			Args:       args,
			Liveness:   "alive",
			Source:     "process",
			Confidence: model.ConfidenceHigh,
			ObservedAt: now,
		})
	}
	return processes, nil
}
