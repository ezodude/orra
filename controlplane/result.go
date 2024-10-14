package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

func NewResultAggregator(dependencies DependencyKeys, logManager *LogManager) LogWorker {
	return &ResultAggregator{
		Dependencies: dependencies,
		LogManager:   logManager,
		logState: &LogState{
			LastOffset:      0,
			Processed:       make(map[string]bool),
			DependencyState: make(map[string]json.RawMessage),
		},
	}
}

func (r *ResultAggregator) Start(ctx context.Context, orchestrationID string) {
	logStream := r.LogManager.GetLog(orchestrationID)
	if logStream == nil {
		r.LogManager.Logger.Error().Str("orchestrationID", orchestrationID).Msg("Log stream not found for orchestration")
		return
	}

	// Channel to receive new log entries
	entriesChan := make(chan LogEntry, 100)

	// Start a goroutine for continuous polling
	go r.PollLog(ctx, logStream, entriesChan)

	// Process entries as they come in
	for {
		select {
		case entry := <-entriesChan:
			if err := r.processEntry(entry, orchestrationID); err != nil {
				r.LogManager.Logger.
					Error().
					Interface("entry", entry).
					Msgf(
						"Result aggregator failed to process entry for orchestration: %s",
						orchestrationID,
					)
				r.LogManager.FailOrchestration(orchestrationID, fmt.Sprintf("Result aggregator failed: %v", err))
				return
			}
		case <-ctx.Done():
			r.LogManager.Logger.Info().Msgf("Result aggregator in orchestration %s is stopping", orchestrationID)
			return
		}
	}
}

func (r *ResultAggregator) PollLog(ctx context.Context, logStream *Log, entriesChan chan<- LogEntry) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			entries := logStream.ReadFrom(r.logState.LastOffset)
			r.LogManager.Logger.Debug().
				Interface("entries", entries).
				Msg("polling log entries for result aggregator in orchestration")

			for _, entry := range entries {
				if !r.shouldProcess(entry) {
					continue
				}

				select {
				case entriesChan <- entry:
					r.logState.LastOffset = entry.Offset + 1
				case <-ctx.Done():
					return
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func (r *ResultAggregator) shouldProcess(entry LogEntry) bool {
	_, isDependency := r.Dependencies[entry.ID]
	return entry.Type == "task_output" && isDependency
}

func (r *ResultAggregator) processEntry(entry LogEntry, orchestrationID string) error {
	// Skip if already processed
	if processed, exists := r.logState.Processed[entry.ID]; exists && processed {
		return nil
	}

	// Store the entry's output in our dependency state
	r.logState.DependencyState[entry.ID] = entry.Value

	if !containsAll(r.logState.DependencyState, r.Dependencies) {
		return nil
	}

	// Mark this entry as processed
	r.logState.Processed[entry.ID] = true
	completed, err := r.LogManager.MarkTaskCompleted(orchestrationID, entry.ID)
	if err != nil {
		return err
	}
	results := r.logState.DependencyState.SortedValues()

	return r.LogManager.FinalizeOrchestration(orchestrationID, completed, "", results[len(results)-1])
}
