package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"time"
)

func NewLogManager(_ context.Context, retention time.Duration, controlPlane *ControlPlane) *LogManager {
	lm := &LogManager{
		logs:           make(map[string]*Log),
		orchestrations: make(map[string]*OrchestrationState),
		retention:      retention,
		cleanupTicker:  time.NewTicker(5 * time.Minute),
		webhookClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		controlPlane: controlPlane,
	}

	//go lm.startCleanup(ctx)
	return lm
}

func (lm *LogManager) startCleanup(ctx context.Context) {
	for {
		select {
		case <-lm.cleanupTicker.C:
			lm.cleanupStaleOrchestrations()
		case <-ctx.Done():
			return
		}
	}
}

func (lm *LogManager) cleanupStaleOrchestrations() {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	now := time.Now()

	for id, orchestrationState := range lm.orchestrations {
		if orchestrationState.Status == Completed &&
			now.Sub(orchestrationState.UpdatedAt) > lm.retention {
			delete(lm.orchestrations, id)
			delete(lm.logs, id)
		}
	}
}

func (lm *LogManager) GetLog(orchestrationID string) *Log {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return lm.logs[orchestrationID]
}

func (lm *LogManager) CreateLog(orchestrationID string, plan *ServiceCallingPlan) *Log {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	log := &Log{
		Entries:      make([]LogEntry, 0),
		lastAccessed: time.Now(),
	}

	state := &OrchestrationState{
		ID:             orchestrationID,
		Plan:           plan,
		CompletedTasks: make(map[string]bool),
		Status:         Processing,
		CreatedAt:      time.Now(),
	}

	lm.logs[orchestrationID] = log
	lm.orchestrations[orchestrationID] = state

	lm.Logger.Debug().Msgf("Created Log for orchestration: %s", orchestrationID)

	return log
}

func (lm *LogManager) MarkTaskCompleted(orchestrationID, taskID string) (Status, error) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	state, ok := lm.orchestrations[orchestrationID]
	if !ok {
		return state.Status, fmt.Errorf("orchestration %s has no associated state", orchestrationID)
	}
	state.CompletedTasks[taskID] = true
	state.UpdatedAt = time.Now()

	return state.Status, nil
}

func (lm *LogManager) MarkOrchestrationCompleted(orchestrationID string) (Status, error) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	state, ok := lm.orchestrations[orchestrationID]
	if !ok {
		return state.Status, fmt.Errorf("orchestration %s has no associated state", orchestrationID)
	}

	state.Status = Completed
	state.UpdatedAt = time.Now()

	return state.Status, nil
}

func (lm *LogManager) MarkOrchestrationFailed(orchestrationID string, reason json.RawMessage) (Status, error) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	state, ok := lm.orchestrations[orchestrationID]
	if !ok {
		return state.Status, fmt.Errorf("orchestration %s has no associated state", orchestrationID)
	}

	state.Error = string(reason)
	state.Status = Failed
	state.UpdatedAt = time.Now()

	return state.Status, nil
}

func (lm *LogManager) GetOrchestrationProjectID(orchestrationID string) string {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return lm.orchestrations[orchestrationID].ProjectID
}

func (lm *LogManager) AppendFailureToLog(orchestrationID, id, producerID, reason string) error {
	reasonData, err := json.Marshal(reason)
	if err != nil {
		return fmt.Errorf("failed to marshal reason for log entry: %w", err)
	}
	// Create a new log entry for our task's output
	newEntry := LogEntry{
		Type:       "task_failure",
		ID:         id,
		Value:      reasonData,
		ProducerID: producerID,
		Timestamp:  time.Now(),
	}

	// Append our output to the log
	if err := lm.GetLog(orchestrationID).Append(newEntry); err != nil {
		return fmt.Errorf("failed to append task output to log: %w", err)
	}

	return nil
}

func (lm *LogManager) FinalizeOrchestration(orchestrationID string, status Status, reason, result json.RawMessage) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if err := lm.controlPlane.FinalizeOrchestration(orchestrationID, status, reason, []json.RawMessage{result}); err != nil {
		return err
	}

	delete(lm.logs, orchestrationID)
	delete(lm.orchestrations, orchestrationID)

	return nil
}

func (l *Log) Append(entry LogEntry) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry.Offset = l.CurrentOffset
	entry.Timestamp = time.Now()

	l.Entries = append(l.Entries, entry)
	l.CurrentOffset += 1
	l.lastAccessed = time.Now()

	return nil
}

func (l *Log) ReadFrom(offset uint64) []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if offset >= l.CurrentOffset {
		return nil
	}

	return l.Entries[offset:]
}

func (l *Log) GetCurrentOffset() uint64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.CurrentOffset
}

func (d DependencyState) SortedValues() []json.RawMessage {
	var out []json.RawMessage
	var keys []string

	for key := range d {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	for _, key := range keys {
		out = append(out, d[key])
	}
	return out
}
