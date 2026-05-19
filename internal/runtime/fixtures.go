package runtime

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fridiculous/the-score/internal/model"
	"github.com/fridiculous/the-score/internal/version"
)

//go:embed testdata/source-pack-processes.json
var sourceFixtureData []byte

type SourceFixtureFile struct {
	SourcePackVersion string               `json:"sourcePackVersion"`
	Cases             []ProcessFixtureCase `json:"cases"`
}

type ProcessFixtureCase struct {
	Name           string           `json:"name"`
	SourceID       string           `json:"sourceId"`
	Description    string           `json:"description,omitempty"`
	Process        model.Process    `json:"process"`
	WantDetected   bool             `json:"wantDetected"`
	WantSource     string           `json:"wantSource,omitempty"`
	WantConfidence model.Confidence `json:"wantConfidence,omitempty"`
	WantStatus     model.Status     `json:"wantStatus,omitempty"`
}

func RunSourceFixtureTests(sourceID string) (model.SourceFixtureReport, error) {
	var file SourceFixtureFile
	if err := json.Unmarshal(sourceFixtureData, &file); err != nil {
		return model.SourceFixtureReport{}, err
	}
	if file.SourcePackVersion != version.SourcePackVersion {
		return model.SourceFixtureReport{}, fmt.Errorf("fixture sourcePackVersion %q does not match runtime %q", file.SourcePackVersion, version.SourcePackVersion)
	}
	report := model.SourceFixtureReport{
		SourcePackVersion: file.SourcePackVersion,
		FilterSourceID:    sourceID,
		Results:           make([]model.SourceFixtureResult, 0, len(file.Cases)),
	}
	observedAt := time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC)
	for _, tc := range file.Cases {
		if sourceID != "" && tc.SourceID != sourceID && tc.WantSource != sourceID {
			continue
		}
		report.Total++
		result := model.SourceFixtureResult{Name: tc.Name, SourceID: tc.SourceID}
		session, ok := InferAgentProcessSession(tc.Process, observedAt)
		switch {
		case ok != tc.WantDetected:
			result.Diagnostic = fmt.Sprintf("detected=%v, want %v", ok, tc.WantDetected)
		case ok && tc.WantSource != "" && session.Source != tc.WantSource:
			result.Diagnostic = fmt.Sprintf("source=%q, want %q", session.Source, tc.WantSource)
		case ok && tc.WantConfidence != "" && session.Confidence != tc.WantConfidence:
			result.Diagnostic = fmt.Sprintf("confidence=%q, want %q", session.Confidence, tc.WantConfidence)
		case ok && tc.WantStatus != "" && session.Status != tc.WantStatus:
			result.Diagnostic = fmt.Sprintf("status=%q, want %q", session.Status, tc.WantStatus)
		default:
			result.Passed = true
			report.Passed++
		}
		if !result.Passed {
			report.Failed++
		}
		report.Results = append(report.Results, result)
	}
	return report, nil
}
