package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/sashabaranov/go-openai"
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

func (p *ControlPlane) handleServiceHealthChange(serviceID string, isHealthy bool) {
	orchestrationsAndTasks := p.GetActiveOrchestrationsAndTasksForService(serviceID)
	if !isHealthy {
		p.LogManager.UpdateActiveOrchestrations(orchestrationsAndTasks, serviceID, "service_unhealthy", Processing, Paused)
		return
	}

	p.LogManager.UpdateActiveOrchestrations(orchestrationsAndTasks, serviceID, "service_healthy", Paused, Processing)
	p.restartOrchestrationTasks(orchestrationsAndTasks)
}

func (p *ControlPlane) GetActiveOrchestrationsAndTasksForService(serviceID string) map[string]map[string]SubTask {
	out := map[string]map[string]SubTask{}
	var projectsForService []string

	p.servicesMu.RLock()
	for projectId, pServices := range p.services {
		for svcId := range pServices {
			if svcId == serviceID {
				projectsForService = append(projectsForService, projectId)
			}
		}
	}
	p.servicesMu.RUnlock()

	p.orchestrationStoreMu.RLock()
	for _, projectId := range projectsForService {
		for _, orchestration := range p.orchestrationStore {
			if orchestration.IsActive() && projectId == orchestration.ProjectID {
				out[orchestration.ID] = orchestration.GetSubTasksFor(serviceID)
			}
		}
	}
	p.orchestrationStoreMu.RUnlock()
	return out
}

func (p *ControlPlane) restartOrchestrationTasks(orchestrationsAndTasks map[string]map[string]SubTask) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for orchestrationID, tasks := range orchestrationsAndTasks {
		for _, task := range tasks {
			completed, err := p.LogManager.IsTaskCompleted(orchestrationID, task.ID)
			if err != nil {
				p.Logger.Error().
					Err(err).
					Str("orchestrationID", orchestrationID).
					Str("taskID", task.ID).
					Msg("failed to check if task is completed during restart - continuing")
			}

			if !completed {
				p.restartTask(orchestrationID, task)
			}
		}
	}
}

func (p *ControlPlane) restartTask(orchestrationID string, task SubTask) {
	if _, oExists := p.logWorkers[orchestrationID]; oExists {
		if cancel, cExists := p.logWorkers[orchestrationID][task.ID]; cExists {
			cancel()
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	p.logWorkers[orchestrationID][task.ID] = cancel

	worker := NewTaskWorker(task.Service, task.ID, task.extractDependencies(), p.LogManager)
	go worker.Start(ctx, orchestrationID)
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

func (p *ControlPlane) PrepareOrchestration(orchestration *Orchestration) {
	p.orchestrationStoreMu.Lock()
	defer p.orchestrationStoreMu.Unlock()

	p.orchestrationStore[orchestration.ID] = orchestration
	services, err := p.discoverProjectServices(orchestration.ProjectID)
	if err != nil {
		p.Logger.Error().
			Str("OrchestrationID", orchestration.ID).
			Err(fmt.Errorf("error discovering services: %w", err))

		orchestration.Status = Failed
		marshaledErr, _ := json.Marshal(err.Error())
		orchestration.Error = marshaledErr
		return
	}

	callingPlan, err := p.decomposeAction(orchestration, services)
	if err != nil {
		p.Logger.Error().
			Str("OrchestrationID", orchestration.ID).
			Err(fmt.Errorf("error decomposing action: %w", err))

		orchestration.Status = Failed
		marshaledErr, _ := json.Marshal(fmt.Sprintf("Error decomposing action: %s", err.Error()))
		orchestration.Error = marshaledErr
		return
	}

	if err := p.validateActionable(callingPlan.Tasks); err != nil {
		orchestration.Plan = callingPlan
		orchestration.Status = NotActionable
		marshaledErr, _ := json.Marshal(err.Error())
		orchestration.Error = marshaledErr
		return
	}

	taskZero, onlyServicesCallingPlan := p.callingPlanMinusTaskZero(callingPlan)
	if taskZero == nil {
		p.Logger.Error().
			Str("OrchestrationID", orchestration.ID).
			Err(fmt.Errorf("error locating task zero in calling plan"))

		orchestration.Plan = callingPlan
		orchestration.Status = Failed
		marshaledErr, _ := json.Marshal(fmt.Sprintf("Error locating task zero in calling plan"))
		orchestration.Error = marshaledErr
		return
	}

	taskZeroInput, err := json.Marshal(taskZero.Input)
	if err != nil {
		orchestration.Status = Failed
		marshaledErr, _ := json.Marshal(fmt.Sprintf("Failed to convert task zero into valid params: %v", err))
		orchestration.Error = marshaledErr
		return
	}

	if err = p.validateInput(services, onlyServicesCallingPlan.Tasks); err != nil {
		orchestration.Status = Failed
		marshaledErr, _ := json.Marshal(fmt.Sprintf("Error validating plan input/output: %s", err.Error()))
		orchestration.Error = marshaledErr
		return
	}

	if err := p.addServiceDetails(services, onlyServicesCallingPlan.Tasks); err != nil {
		orchestration.Status = Failed
		marshaledErr, _ := json.Marshal(fmt.Sprintf("Error adding service details to calling plan: %s", err.Error()))
		orchestration.Error = marshaledErr
		return
	}

	orchestration.Plan = onlyServicesCallingPlan
	orchestration.taskZero = taskZeroInput
}

func (p *ControlPlane) ExecuteOrchestration(orchestration *Orchestration) {
	p.Logger.Debug().Msgf("About to create Log for orchestration %s", orchestration.ID)
	log := p.LogManager.PrepLogForOrchestration(orchestration.ID, orchestration.Plan)

	p.Logger.Debug().Msgf("About to create and start workers for orchestration %s", orchestration.ID)
	p.createAndStartWorkers(orchestration.ID, orchestration.Plan)

	initialEntry := NewLogEntry("task_output", TaskZero, orchestration.taskZero, "control-panel", 0)

	p.Logger.Debug().Msgf("About to append initial entry to Log for orchestration %s", orchestration.ID)
	if err := log.Append(initialEntry); err != nil {
		p.Logger.Error().
			Str("OrchestrationID", orchestration.ID).
			Err(fmt.Errorf("error appending initial entry: %w", err))
		return
	}
}

func (p *ControlPlane) FinalizeOrchestration(
	orchestrationID string,
	status Status,
	reason json.RawMessage,
	results []json.RawMessage) error {
	p.orchestrationStoreMu.Lock()
	defer p.orchestrationStoreMu.Unlock()

	orchestration, exists := p.orchestrationStore[orchestrationID]
	if !exists {
		return fmt.Errorf("control panel cannot finalize missing orchestration %s", orchestrationID)
	}

	orchestration.Status = status
	orchestration.Error = reason
	orchestration.Results = results

	p.Logger.Debug().
		Str("OrchestrationID", orchestration.ID).
		Msgf("About to FinalizeOrchestration with status: %s", orchestration.Status.String())

	p.cleanupLogWorkers(orchestration.ID)

	if err := p.triggerWebhook(orchestration); err != nil {
		return fmt.Errorf("failed to trigger webhook for orchestration %s: %w", orchestration.ID, err)
	}

	return nil
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

	if len(resultDependencies) == 0 {
		p.Logger.Error().
			Fields(map[string]any{
				"Dependencies":    resultDependencies,
				"OrchestrationID": orchestrationID,
			}).
			Msg("Result Aggregator has no dependencies")

		return
	}

	p.Logger.Debug().
		Fields(map[string]any{
			"Dependencies":    resultDependencies,
			"OrchestrationID": orchestrationID,
		}).
		Msg("Result Aggregator extracted dependencies")

	aggregator := NewResultAggregator(resultDependencies, p.LogManager)
	ctx, cancel := context.WithCancel(context.Background())
	p.logWorkers[orchestrationID][ResultAggregatorID] = cancel

	fTracker := NewFailureTracker(p.LogManager)
	fCtx, fCancel := context.WithCancel(context.Background())
	p.logWorkers[orchestrationID][FailureTrackerID] = fCancel

	p.Logger.Debug().Str("orchestrationID", orchestrationID).Msg("Starting result aggregator for orchestration")
	go aggregator.Start(ctx, orchestrationID)

	p.Logger.Debug().Str("orchestrationID", orchestrationID).Msg("Starting failure tracker for orchestration")
	go fTracker.Start(fCtx, orchestrationID)
}

func (p *ControlPlane) cleanupLogWorkers(orchestrationID string) {
	p.workerMu.Lock()
	defer p.workerMu.Unlock()

	if cancelFns, exists := p.logWorkers[orchestrationID]; exists {
		for logWorker, cancel := range cancelFns {
			p.Logger.Debug().
				Str("LogWorker", logWorker).
				Msgf("Stopping Log worker for orchestration: %s", orchestrationID)

			cancel() // This will trigger ctx.Done() in the worker
		}
		delete(p.logWorkers, orchestrationID)
		p.Logger.Debug().
			Str("OrchestrationID", orchestrationID).
			Msg("Cleaned up task workers for orchestration.")
	}
}

func (p *ControlPlane) callingPlanMinusTaskZero(callingPlan *ServiceCallingPlan) (*SubTask, *ServiceCallingPlan) {
	var taskZero *SubTask
	var serviceTasks []*SubTask

	for _, subTask := range callingPlan.Tasks {
		if strings.EqualFold(subTask.ID, TaskZero) {
			taskZero = subTask
			continue
		}
		serviceTasks = append(serviceTasks, subTask)
	}

	return taskZero, &ServiceCallingPlan{
		ProjectID:      callingPlan.ProjectID,
		Tasks:          serviceTasks,
		ParallelGroups: callingPlan.ParallelGroups,
	}
}

func (p *ControlPlane) validateActionable(subTasks []*SubTask) error {
	for _, subTask := range subTasks {
		if strings.EqualFold(subTask.ID, "final") {
			return fmt.Errorf("%s", subTask.Error)
		}
	}
	return nil
}

func (p *ControlPlane) triggerWebhook(orchestration *Orchestration) error {
	project, ok := p.projects[orchestration.ProjectID]
	if !ok {
		return fmt.Errorf("project %s not found", orchestration.ProjectID)
	}

	var payload = struct {
		OrchestrationID string            `json:"orchestrationId"`
		Results         []json.RawMessage `json:"results"`
		Status          Status            `json:"status"`
		Error           json.RawMessage   `json:"error,omitempty"`
	}{
		OrchestrationID: orchestration.ID,
		Results:         orchestration.Results,
		Status:          orchestration.Status,
		Error:           orchestration.Error,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to trigger webhook failed to marshal payload: %w", err)
	}

	p.Logger.Debug().
		Fields(struct {
			OrchestrationID string
			ProjectID       string
			Webhook         string
			Payload         string
		}{
			OrchestrationID: orchestration.ID,
			ProjectID:       project.ID,
			Webhook:         project.Webhook,
			Payload:         string(jsonPayload),
		}).
		Msg("Triggering webhook")

	// Create a new request
	req, err := http.NewRequest("POST", project.Webhook, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Orra/1.0")

	// Create an HTTP client with a timeout
	client := &http.Client{
		Timeout: time.Second * 10,
	}

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			p.Logger.Error().
				Str("OrchestrationID", orchestration.ID).
				Err(fmt.Errorf("failed to close response body when triggering Webhook: %w", err))
		}
	}(resp.Body)

	// Check the response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func (s ServiceSchema) InputIncludes(src string) bool {
	return s.Input.IncludesProp(src)
}

func (s Spec) IncludesProp(src string) bool {
	_, ok := s.Properties[src]
	return ok
}

func (s Spec) String() (string, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (a ActionParams) String() (string, error) {
	data, err := json.Marshal(a)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (si *ServiceInfo) String() string {
	return fmt.Sprintf("[%s] %s - %s", si.Type.String(), si.Name, si.Description)
}

func sanitizeJSONOutput(input string) string {
	// Remove leading and trailing whitespace
	trimmed := strings.TrimSpace(input)

	// Check if the input starts and ends with JSON markdown markers
	if strings.HasPrefix(trimmed, "```json") && strings.HasSuffix(trimmed, "```") {
		// Remove the starting "```json" marker
		withoutStart := strings.TrimPrefix(trimmed, "```json")

		// Remove the ending "```" marker
		withoutEnd := strings.TrimSuffix(withoutStart, "```")

		// Trim any remaining whitespace
		return strings.TrimSpace(withoutEnd)
	}

	// If the input doesn't have the expected markers, return it as is
	return input
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
