package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/google/uuid"
)

var (
	maxRetries               = 5
	maxExecutionTimeOutDelay = 30 * time.Second
)

type RetryableError struct {
	Err error
}

func (e RetryableError) Error() string {
	return fmt.Sprintf("retryable error: %v", e.Err)
}

func NewTaskWorker(serviceID string, taskID string, dependencies DependencyKeys, logManager *LogManager) LogWorker {
	expBackoff := backoff.NewExponentialBackOff()
	expBackoff.MaxElapsedTime = 10 * time.Minute

	return &TaskWorker{
		ServiceID:    serviceID,
		TaskID:       taskID,
		Dependencies: dependencies,
		LogManager:   logManager,
		logState: &LogState{
			LastOffset:      0,
			Processed:       make(map[string]bool),
			DependencyState: make(map[string]json.RawMessage),
		},
		backOff: expBackoff,
	}
}

func (w *TaskWorker) Start(ctx context.Context, orchestrationID string) {
	logStream := w.LogManager.GetLog(orchestrationID)
	if logStream == nil {
		w.LogManager.Logger.Debug().Str("orchestrationID", orchestrationID).Msg("Log stream not found for orchestration")
		return
	}

	// Channel to receive new log entries
	entriesChan := make(chan LogEntry, 100)

	// Start a goroutine for continuous polling
	go w.PollLog(ctx, orchestrationID, logStream, entriesChan)

	// Process entries as they come in
	for {
		select {
		case entry := <-entriesChan:
			if err := w.processEntry(ctx, entry, orchestrationID); err != nil {
				w.LogManager.Logger.
					Error().
					Interface("entry", entry).
					Msgf(
						"Task worker %s failed to process entry for orchestration: %s",
						w.TaskID,
						orchestrationID,
					)
				return
			}
		case <-ctx.Done():
			w.LogManager.Logger.Info().Msgf("TaskWorker for task %s in orchestration %s is stopping", w.TaskID, orchestrationID)
			return
		}
	}
}

func (w *TaskWorker) PollLog(ctx context.Context, _ string, logStream *Log, entriesChan chan<- LogEntry) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			var processableEntries []LogEntry

			entries := logStream.ReadFrom(w.logState.LastOffset)
			for _, entry := range entries {
				if !w.shouldProcess(entry) {
					continue
				}

				processableEntries = append(processableEntries, entry)
				select {
				case entriesChan <- entry:
					w.logState.LastOffset = entry.Offset() + 1
				case <-ctx.Done():
					return
				}
			}

			//w.LogManager.Logger.Debug().
			//	Interface("entries", processableEntries).
			//	Msgf("polling entries for task %s - orchestration %s", w.TaskID, orchestrationID)
		case <-ctx.Done():
			return
		}
	}
}

func (w *TaskWorker) shouldProcess(entry LogEntry) bool {
	_, isDependency := w.Dependencies[entry.ID()]
	processed := w.logState.Processed[entry.ID()]
	return entry.Type() == "task_output" && isDependency && !processed
}

func (w *TaskWorker) processEntry(ctx context.Context, entry LogEntry, orchestrationID string) error {
	// Store the entry's output in our dependency state
	w.logState.DependencyState[entry.ID()] = entry.Value()

	if !containsAll(w.logState.DependencyState, w.Dependencies) {
		return nil
	}

	// Execute our task
	output, err := w.executeTaskWithRetry(ctx, orchestrationID)
	if err != nil {
		w.LogManager.Logger.Error().Err(err).Msgf("Cannot execute task %s for orchestration %s", w.TaskID, orchestrationID)
		return w.LogManager.AppendFailureToLog(orchestrationID, w.TaskID, w.ServiceID, err.Error())
	}

	// Mark this entry as processed
	w.logState.Processed[entry.ID()] = true

	if err := w.LogManager.MarkTaskCompleted(orchestrationID, entry.ID()); err != nil {
		w.LogManager.Logger.Error().Err(err).Msgf("Cannot mark task %s completed for orchestration %s", w.TaskID, orchestrationID)
		return w.LogManager.AppendFailureToLog(orchestrationID, w.TaskID, w.ServiceID, err.Error())
	}

	// Create a new log entry for our task's output
	newEntry := NewLogEntry("task_output", w.TaskID, output, w.ServiceID, 0)

	// Append our output to the log
	if err := w.LogManager.GetLog(orchestrationID).Append(newEntry); err != nil {
		w.LogManager.Logger.Error().Err(err).Msgf("Cannot append task %s output to Log for orchestration %s", w.TaskID, orchestrationID)
		return w.LogManager.AppendFailureToLog(
			orchestrationID,
			w.TaskID,
			w.ServiceID,
			fmt.Errorf("failed to append task output to log: %w", err).Error())
	}

	return nil
}

func (w *TaskWorker) executeTaskWithRetry(ctx context.Context, orchestrationID string) (json.RawMessage, error) {
	var result json.RawMessage
	var consecutiveFailures int

	operation := func() error {
		paused, err := w.LogManager.IsTaskPaused(orchestrationID, w.TaskID)
		if err != nil {
			return backoff.Permanent(fmt.Errorf("failed to get orchestration status: %v", err))
		}

		if paused {
			w.LogManager.Logger.Debug().
				Str("TaskID", w.TaskID).
				Msgf("task is paused for orchestration: %s", orchestrationID)
			return backoff.Permanent(fmt.Errorf("task is paused"))
		}

		//if !w.LogManager.controlPlane.WebSocketManager.IsServiceHealthy(w.ServiceID) {
		//	return RetryableError{Err: fmt.Errorf("service is unhealthy")}
		//}

		result, err = w.executeTask(ctx, orchestrationID)
		if err != nil {
			consecutiveFailures++
			if consecutiveFailures > maxRetries {
				return backoff.Permanent(fmt.Errorf("too many consecutive failures: %w", err))
			}
			if isRetryableError(err) {
				return RetryableError{Err: err}
			}
			return backoff.Permanent(err)
		}

		consecutiveFailures = 0
		return nil
	}

	err := backoff.RetryNotify(operation, w.backOff, func(err error, duration time.Duration) {
		if retryErr, ok := err.(RetryableError); ok {
			w.LogManager.Logger.Info().
				Str("OrchestrationID", orchestrationID).
				Str("TaskID", w.TaskID).
				Err(retryErr.Err).
				Dur("RetryAfter", duration).
				Msg("Retrying task due to retryable error")
		}
	})

	if err != nil {
		return nil, err
	}

	successEntry := NewLogEntry("task_output", w.TaskID, result, w.ServiceID, 0)
	if err := w.LogManager.GetLog(orchestrationID).Append(successEntry); err != nil {
		return nil, err
	}

	return result, nil
}

func (w *TaskWorker) executeTask(ctx context.Context, orchestrationID string) (json.RawMessage, error) {
	input, err := mergeValueMapsToJson(w.logState.DependencyState)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input for task %s: %w", w.TaskID, err)
	}

	// Generate a unique execution ID
	executionID := uuid.New().String()

	task := &Task{
		ID:              w.TaskID,
		ExecutionID:     executionID,
		Input:           input,
		ServiceID:       w.ServiceID,
		OrchestrationID: orchestrationID,
		ProjectID:       w.LogManager.GetOrchestrationProjectID(orchestrationID),
		Status:          Processing,
	}

	resultChan := make(chan json.RawMessage, 1)
	errChan := make(chan error, 1)

	w.LogManager.controlPlane.WebSocketManager.RegisterTaskCallback(executionID, func(result json.RawMessage, err error) {

		fields := map[string]any{
			"executionID": executionID,
			"result":      string(result),
		}

		if err != nil {
			fields["error"] = err.Error()
			errChan <- err
		} else {
			resultChan <- result
		}

		w.LogManager.Logger.Debug().
			Fields(fields).
			Msgf("Triggered a task callback: %s, for serviceID: %s", w.TaskID, w.ServiceID)
	})

	if err := w.LogManager.controlPlane.WebSocketManager.SendTask(w.ServiceID, task); err != nil {
		return nil, fmt.Errorf("failed to send task %s for service %s: %w", task.ID, w.ServiceID, err)
	}

	select {
	case result := <-resultChan:
		return result, nil
	case err := <-errChan:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(maxExecutionTimeOutDelay * time.Second):
		w.LogManager.controlPlane.WebSocketManager.UnregisterTaskCallback(executionID)
		return nil, fmt.Errorf("task execution timed out")
	}
}

func mergeValueMapsToJson(src map[string]json.RawMessage) (json.RawMessage, error) {
	out := make(map[string]any)
	for _, input := range src {
		if err := json.Unmarshal(input, &out); err != nil {
			return nil, err
		}
	}
	return json.Marshal(out)
}

func containsAll(s map[string]json.RawMessage, e map[string]struct{}) bool {
	for srcId := range e {
		if _, hasOutput := s[srcId]; !hasOutput {
			return false
		}
	}
	return true
}

func isRetryableError(err error) bool {
	// Implement logic to determine if the error is retryable
	// For example, timeouts and connection errors might be retryable,
	// while validation errors might not be.
	// This is a simplified example:
	errorMsg := strings.ToLower(err.Error())
	return strings.Contains(errorMsg, "task execution timed out") ||
		strings.Contains(errorMsg, "failed to send task") ||
		strings.Contains(errorMsg, "failed to read result") ||
		strings.Contains(errorMsg, "rate limit exceeded")
}
