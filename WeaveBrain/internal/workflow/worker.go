package workflow

import (
	"log"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

// Worker wraps a Temporal worker for WeaveBrain workflows.
type Worker struct {
	worker worker.Worker
}

// StartWorker creates and starts a Temporal worker that registers all workflows and activities.
func StartWorker(c client.Client, acts *Activities) *Worker {
	// Set package-level activities for standalone functions
	SetActivities(acts)

	w := worker.New(c, TaskQueue, worker.Options{})

	// Register workflows
	w.RegisterWorkflow(IdeaWorkflow)
	w.RegisterWorkflow(ReminderWorkflow)
	w.RegisterWorkflow(BatchAggregationWorkflow)

	// Register activities (standalone functions)
	w.RegisterActivity(ProcessIdeaActivity)
	w.RegisterActivity(SaveIdeaActivity)
	w.RegisterActivity(GetPendingRemindersActivity)
	w.RegisterActivity(TriggerReminderActivity)
	w.RegisterActivity(AggregateDailyIdeasActivity)

	go func() {
		log.Println("[Temporal] Worker starting on queue:", TaskQueue)
		if err := w.Run(worker.InterruptCh()); err != nil {
			log.Printf("[Temporal] Worker stopped: %v", err)
		}
	}()

	return &Worker{worker: w}
}

// Stop gracefully stops the Temporal worker.
func (w *Worker) Stop() {
	w.worker.Stop()
}
