package service

import (
	"context"
	"errors"
	"time"

	chimetrics "github.com/go-chi/metrics"

	"github.com/belyaevedu/remote-code-service/internal/domain"
	"github.com/belyaevedu/remote-code-service/internal/port"
)

const (
	taskStatusSuccess = "success" // exit code 0
	taskStatusFailure = "failure" // ran but the user code failed
	taskStatusTimeout = "timeout" // killed by the execution timout
	taskStatusError   = "error"   // infra failure
)

type taskLabels struct {
	Translator string `label:"translator"`
	Status     string `label:"status"`
}

type taskInFlightLabels struct {
	Translator string `label:"translator"`
}

var (
	tasksInFlight = chimetrics.GaugeWith[taskInFlightLabels](
		"tasks_in_flight",
		"Number of code tasks currently executing in the sandbox.",
	)
	tasksProcessedTotal = chimetrics.CounterWith[taskLabels](
		"tasks_processed_total",
		"Total number of code execution tasks processed by the worker.",
	)
	taskExecutionDuration = chimetrics.HistogramWith[taskLabels](
		"task_execution_duration_seconds",
		"Time a code task spent executing in the sandbox.",
		[]float64{.05, .1, .25, .5, 1, 2.5, 5, 10, 15, 30, 60},
	)
)

type InstrumentedExecutor struct {
	inner port.CodeExecutor
}

var _ port.CodeExecutor = (*InstrumentedExecutor)(nil)

func NewInstrumentedExecutor(inner port.CodeExecutor) *InstrumentedExecutor {
	return &InstrumentedExecutor{inner: inner}
}

func (e *InstrumentedExecutor) Execute(ctx context.Context, msg domain.TaskMessage) (domain.ExecutionResult, error) {
	inflight := taskInFlightLabels{Translator: msg.Translator}
	tasksInFlight.Inc(inflight)
	defer tasksInFlight.Dec(inflight)

	start := time.Now()
	result, err := e.inner.Execute(ctx, msg)
	duration := time.Since(start)

	status := taskStatusSuccess
	switch {
	case err != nil:
		status = taskStatusError
		if errors.Is(err, domain.ErrExecutionTimeout) {
			status = taskStatusTimeout
		}
	case result.Failed:
		status = taskStatusFailure
	}

	labels := taskLabels{Translator: msg.Translator, Status: status}
	tasksProcessedTotal.Inc(labels)
	taskExecutionDuration.Observe(duration.Seconds(), labels)

	return result, err
}
