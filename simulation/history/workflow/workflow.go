package workflow

import (
	"time"

	"go.uber.org/cadence/worker"
	"go.uber.org/cadence/workflow"
)

func RegisterWorker(w worker.Registry) {
	w.RegisterWorkflow(SimulationWorkflow)
}

func SimulationWorkflow(ctx workflow.Context, numSleeps int) error {
	for i := 0; i < numSleeps; i++ {
		workflow.Sleep(ctx, time.Second)
	}
	return nil
}
