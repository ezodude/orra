package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

func NewControlPlane(openAIKey string) *ControlPlane {
	plane := &ControlPlane{
		projects:           make(map[string]*Project),
		services:           make(map[string]map[string]*ServiceInfo),
		orchestrationStore: make(map[string]*Orchestration),
		logWorkers:         make(map[string]map[string]context.CancelFunc),
		openAIKey:          openAIKey,
	}
	return plane
}

func (p *ControlPlane) Initialise(ctx context.Context, logMgr *LogManager, wsManager *WebSocketManager, coordinator *HealthCoordinator, Logger zerolog.Logger) {
	p.LogManager = logMgr
	p.Logger = Logger
	p.WebSocketManager = wsManager
	p.WebSocketManager.RegisterHealthCallback(coordinator.handleServiceHealthChange)
	p.TidyWebSocketArtefacts(ctx)
}

func (p *ControlPlane) TidyWebSocketArtefacts(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				p.WebSocketManager.CleanupExpiredMessages()
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (p *ControlPlane) RegisterOrUpdateService(service *ServiceInfo) error {
	p.servicesMu.Lock()
	defer p.servicesMu.Unlock()

	projectServices, exists := p.services[service.ProjectID]
	if !exists {
		p.Logger.Debug().
			Str("ProjectID", service.ProjectID).
			Str("ServiceName", service.Name).
			Msgf("Creating new project service")
		projectServices = make(map[string]*ServiceInfo)
		p.services[service.ProjectID] = projectServices
	}

	if len(strings.TrimSpace(service.ID)) == 0 {
		service.ID = p.generateServiceKey(service.ProjectID)
		service.Version = 1

		p.Logger.Debug().
			Str("ProjectID", service.ProjectID).
			Str("ServiceName", service.Name).
			Msgf("Generating new service ID")
	} else {
		existingService, exists := projectServices[service.ID]
		if !exists {
			return fmt.Errorf("service with key %s not found in project %s", service.ID, service.ProjectID)
		}
		service.ID = existingService.ID
		service.Version = existingService.Version + 1

		p.Logger.Debug().
			Str("ProjectID", service.ProjectID).
			Str("ServiceID", service.ID).
			Str("ServiceName", service.Name).
			Int64("ServiceVersion", service.Version).
			Msgf("Updating existing service")
	}
	projectServices[service.ID] = service

	return nil
}

func (p *ControlPlane) GetServiceName(projectID string, serviceID string) (string, error) {
	p.servicesMu.RLock()
	defer p.servicesMu.RUnlock()

	projectServices := p.services[projectID]
	svc, exists := projectServices[serviceID]
	if !exists {
		return "", fmt.Errorf("service %s not found for project %s", serviceID, projectID)
	}
	return svc.Name, nil
}

func (p *ControlPlane) GetProjectByApiKey(key string) (*Project, error) {
	apiKeyToProjectID := make(map[string]string)
	for id, project := range p.projects {
		apiKeyToProjectID[project.APIKey] = id
	}

	if projectID, exists := apiKeyToProjectID[key]; exists {
		return p.projects[projectID], nil
	} else {
		return nil, fmt.Errorf("no project found with the given API key: %s", key)
	}
}

func (p *ControlPlane) ServiceBelongsToProject(svcID, projectID string) bool {
	p.servicesMu.RLock()
	defer p.servicesMu.RUnlock()

	projectServices, exists := p.services[projectID]
	if !exists {
		return false
	}
	_, ok := projectServices[svcID]
	return ok
}

func (p *ControlPlane) generateServiceKey(projectID string) string {
	// Generate a unique key for the service
	// This could be a UUID, a hash of project ID + timestamp, or any other method
	// that ensures uniqueness within the project
	return fmt.Sprintf("%s-%s", projectID, uuid.New().String())
}

func (p *ControlPlane) ProjectsForService(serviceID string) []string {
	p.servicesMu.RLock()
	defer p.servicesMu.RUnlock()
	var out []string
	for projectId, pServices := range p.services {
		for svcId := range pServices {
			if svcId == serviceID {
				out = append(out, projectId)
			}
		}
	}
	return out
}

func (p *ControlPlane) ActiveOrchestrationsWithTasks(projects []string, serviceID string) map[string]map[string]SubTask {
	p.orchestrationStoreMu.RLock()
	defer p.orchestrationStoreMu.RUnlock()
	out := map[string]map[string]SubTask{}
	for _, projectId := range projects {
		for _, orchestration := range p.orchestrationStore {
			if orchestration.IsActive() && projectId == orchestration.ProjectID {
				out[orchestration.ID] = orchestration.GetSubTasksFor(serviceID)
			}
		}
	}
	return out
}

func (p *ControlPlane) StopTaskWorker(orchestrationID string, taskID string) {
	p.workerMu.Lock()
	defer p.workerMu.Unlock()

	if _, ok := p.logWorkers[orchestrationID]; !ok {
		return
	}

	cancel, ok := p.logWorkers[orchestrationID][taskID]
	if !ok {
	}

	cancel()
}

func (p *ControlPlane) CreateAndStartTaskWorker(orchestrationID string, task SubTask) {
	p.workerMu.Lock()
	defer p.workerMu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	p.logWorkers[orchestrationID][task.ID] = cancel

	worker := NewTaskWorker(task.Service, task.ID, task.extractDependencies(), p.LogManager)
	go worker.Start(ctx, orchestrationID)
}

func (o *Orchestration) GetSubTasksFor(serviceID string) map[string]SubTask {
	out := map[string]SubTask{}
	for _, subTask := range o.Plan.Tasks {
		if strings.EqualFold(subTask.Service, serviceID) {
			out[subTask.ID] = SubTask{
				ID:             subTask.ID,
				Service:        subTask.Service,
				ServiceDetails: subTask.ServiceDetails,
				Input:          subTask.Input,
				Status:         subTask.Status,
				Error:          subTask.Error,
			}
		}
	}
	return out
}

func (o *Orchestration) Executable() bool {
	return o.Status != NotActionable && o.Status != Failed
}

func (o *Orchestration) IsActive() bool {
	return o.Status == Processing || o.Status == Paused
}

func (s *SubTask) extractDependencies() DependencyKeys {
	out := make(DependencyKeys)
	for _, source := range s.Input {
		if dep := extractDependencyID(string(source)); dep != "" {
			out[dep] = struct{}{}
		}
	}
	return out
}

// extractDependencyID extracts the task ID from a dependency reference
// Example: "$task0.param1" returns "task0"
func extractDependencyID(input string) string {
	matches := DependencyPattern.FindStringSubmatch(input)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}
