package main

import (
	"github.com/rs/zerolog"
)

func NewHealthCoordinator(plane *ControlPlane, manager *LogManager, logger zerolog.Logger) *HealthCoordinator {
	return &HealthCoordinator{
		plane:           plane,
		logManager:      manager,
		logger:          logger,
		lastHealthState: make(map[string]bool),
	}
}

func (h *HealthCoordinator) handleServiceHealthChange(serviceID string, isHealthy bool) {
	if health, exists := h.lastHealthState[serviceID]; exists && health == isHealthy {
		return
	}

	h.lastHealthState[serviceID] = isHealthy

	orchestrationsAndTasks := h.GetActiveOrchestrationsAndTasksForService(serviceID)
	if !isHealthy {
		h.logManager.UpdateActiveOrchestrations(orchestrationsAndTasks, serviceID, "service_unhealthy", Processing, Paused)
		return
	}

	h.logManager.UpdateActiveOrchestrations(orchestrationsAndTasks, serviceID, "service_healthy", Paused, Processing)
	h.restartOrchestrationTasks(orchestrationsAndTasks)
}

func (h *HealthCoordinator) GetActiveOrchestrationsAndTasksForService(serviceID string) map[string]map[string]SubTask {
	h.mu.RLock()
	defer h.mu.RUnlock()

	projectID, err := h.plane.GetProjectIDForService(serviceID)
	if err != nil {
		h.logger.Error().Err(err).Str("serviceID", serviceID).Msg("Failed to get project ID from control plane")
		return map[string]map[string]SubTask{}
	}

	return h.plane.ActiveOrchestrationsWithTasks(projectID, serviceID)
}

func (h *HealthCoordinator) restartOrchestrationTasks(orchestrationsAndTasks map[string]map[string]SubTask) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for orchestrationID, tasks := range orchestrationsAndTasks {
		for _, task := range tasks {
			completed, err := h.logManager.IsTaskCompleted(orchestrationID, task.ID)
			if err != nil {
				h.logger.Error().
					Err(err).
					Str("orchestrationID", orchestrationID).
					Str("taskID", task.ID).
					Msg("failed to check if task is completed during restart - continuing")
			}

			if !completed {
				h.restartTask(orchestrationID, task)
			}
		}
	}
}

func (h *HealthCoordinator) restartTask(orchestrationID string, task SubTask) {
	h.logger.Debug().
		Str("orchestrationID", orchestrationID).
		Str("taskID", task.ID).
		Msg("Restarting task")

	h.plane.StopTaskWorker(orchestrationID, task.ID)
	h.plane.CreateAndStartTaskWorker(orchestrationID, task)
}
