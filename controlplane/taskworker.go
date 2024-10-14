package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

func NewTaskWorker(serviceID string, taskID string, dependencies DependencyKeys, logManager *LogManager) LogWorker {
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
	go w.PollLog(ctx, logStream, entriesChan)

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
				w.LogManager.FailOrchestration(orchestrationID, fmt.Sprintf("Task %s failed: %v", w.TaskID, err))
				return
			}
		case <-ctx.Done():
			w.LogManager.Logger.Info().Msgf("TaskWorker for task %s in orchestration %s is stopping", w.TaskID, orchestrationID)
			return
		}
	}
}

func (w *TaskWorker) PollLog(ctx context.Context, logStream *Log, entriesChan chan<- LogEntry) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			entries := logStream.ReadFrom(w.logState.LastOffset)
			w.LogManager.Logger.Debug().
				Interface("entries", entries).
				Msgf("polling log entries for task %s", w.TaskID)

			for _, entry := range entries {
				if !w.shouldProcess(entry) {
					continue
				}

				select {
				case entriesChan <- entry:
					w.logState.LastOffset = entry.Offset + 1
				case <-ctx.Done():
					return
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func (w *TaskWorker) shouldProcess(entry LogEntry) bool {
	_, isDependency := w.Dependencies[entry.ID]
	return entry.Type == "task_output" && isDependency
}

func (w *TaskWorker) processEntry(ctx context.Context, entry LogEntry, orchestrationID string) error {
	// Skip if already processed
	if processed, exists := w.logState.Processed[entry.ID]; exists && processed {
		return nil
	}

	// Store the entry's output in our dependency state
	w.logState.DependencyState[entry.ID] = entry.Value

	if !containsAll(w.logState.DependencyState, w.Dependencies) {
		return nil
	}

	// Execute our task
	output, err := w.executeTask(ctx, orchestrationID)
	if err != nil {
		return fmt.Errorf("failed to execute task: %w", err)
	}

	// Mark this entry as processed
	w.logState.Processed[entry.ID] = true

	// Create a new log entry for our task's output
	newEntry := LogEntry{
		Type:       "task_output",
		ID:         w.TaskID,
		Value:      output,
		ProducerID: w.ServiceID,
		Timestamp:  time.Now(),
	}

	// Append our output to the log
	if err := w.LogManager.GetLog(orchestrationID).Append(newEntry); err != nil {
		return fmt.Errorf("failed to append task output to log: %w", err)
	}

	//return w.LogManager.checkOrchestrationCompletion(orchestrationID)
	return nil
}

func (w *TaskWorker) executeTask(ctx context.Context, orchestrationID string) (json.RawMessage, error) {
	input, err := mergeValueMapsToJson(w.logState.DependencyState)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input for task %s: %w", w.TaskID, err)
	}

	svcConn := w.LogManager.controlPlane.GetServiceConnection(w.ServiceID)
	if svcConn == nil {
		return nil, fmt.Errorf("ServiceID %s has no connection", w.ServiceID)
	}

	task := &Task{
		ID:              w.TaskID,
		Input:           input,
		ServiceID:       w.ServiceID,
		OrchestrationID: orchestrationID,
		ProjectID:       w.LogManager.GetOrchestrationProjectID(orchestrationID),
		Status:          Processing,
	}

	resultChan := make(chan json.RawMessage, 1)
	errChan := make(chan error, 1)

	go func() {
		if err := svcConn.Conn.WriteJSON(task); err != nil {
			errChan <- fmt.Errorf("failed to send task: %w", err)
			return
		}

		var result struct {
			TaskID string          `json:"taskId"`
			Result json.RawMessage `json:"result"`
		}

		if err := svcConn.Conn.ReadJSON(&result); err != nil {
			errChan <- fmt.Errorf("failed to read result: %w", err)
			return
		}

		resultChan <- result.Result
	}()

	select {
	case result := <-resultChan:
		return result, nil
	case err := <-errChan:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("task execution timed out")
	}
}

func (w *TaskWorker) saveState(orchestrationID string) {
	w.stateMu.Lock()
	defer w.stateMu.Unlock()

	stateKey := fmt.Sprintf("worker_state_%s_%s", orchestrationID, w.ServiceID)
	_, err := json.Marshal(w.logState)
	if err != nil {
		w.LogManager.Logger.Error().Msgf("Failed to marshal worker state: %v", err)
		return
	}

	// Here you would typically save this to a persistent store
	// For this example, we'll just log it
	w.LogManager.Logger.Debug().Msgf("Saved worker state: %s", stateKey)
}

func (w *TaskWorker) loadState(orchestrationID string) {
	w.stateMu.Lock()
	defer w.stateMu.Unlock()

	stateKey := fmt.Sprintf("worker_state_%s_%s", orchestrationID, w.ServiceID)

	// Here you would typically load this from a persistent store
	// For this example, we'll just log it
	w.LogManager.Logger.Debug().Msgf("Loaded worker state: %s", stateKey)

	// If you had actual state data, you would unmarshal it like this:
	// err := json.Unmarshal(stateData, &w.workerState)
	// if err != nil {
	//     log.Printf("Failed to unmarshal worker state: %v", err)
	// }
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
