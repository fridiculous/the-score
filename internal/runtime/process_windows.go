//go:build windows

package runtime

import (
	"encoding/csv"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/fridiculous/the-score/internal/model"
)

type SystemProcessLister struct{}

func (SystemProcessLister) ListProcesses() ([]model.Process, error) {
	out, err := exec.Command("tasklist", "/FO", "CSV", "/NH").Output()
	if err != nil {
		return nil, err
	}
	reader := csv.NewReader(strings.NewReader(string(out)))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	processes := make([]model.Process, 0, len(records))
	for _, rec := range records {
		if len(rec) < 2 {
			continue
		}
		pid, err := strconv.Atoi(rec[1])
		if err != nil {
			continue
		}
		processes = append(processes, model.Process{
			ID:         "process:" + strconv.Itoa(pid),
			Kind:       "process",
			PID:        pid,
			Command:    rec[0],
			Liveness:   "alive",
			Source:     "process",
			Confidence: model.ConfidenceMedium,
			ObservedAt: now,
		})
	}
	return processes, nil
}
