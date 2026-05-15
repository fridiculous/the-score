package runtime

import "github.com/fridiculous/the-score/internal/model"

type ProcessLister interface {
	ListProcesses() ([]model.Process, error)
}
