package workflow

import (
	"go.uber.org/cadence/worker"
	"go.uber.org/cadence/workflow"
)

func RegisterWorker(w worker.Registry) {
	w.RegisterWorkflow(NoopWorkflow)
}

func NoopWorkflow(ctx workflow.Context) error {
	return nil
}
