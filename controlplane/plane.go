package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sashabaranov/go-openai"
)

func NewControlPlane(openAIKey string) *ControlPlane {
	return &ControlPlane{
		wsConnections:      make(map[string]*ServiceConnection),
		projects:           make(map[string]*Project),
		services:           make(map[string][]*ServiceInfo),
		orchestrationStore: make(map[string]*Orchestration),
		openAIKey:          openAIKey,
	}
}

func (p *ControlPlane) discoverProjectServices(projectID string) ([]*ServiceInfo, error) {
	services, ok := p.services[projectID]
	if !ok {
		return nil, fmt.Errorf("no services found for project %s", projectID)
	}
	return services, nil
}

func (p *ControlPlane) generateLLMPrompt(orchestration *Orchestration, services []*ServiceInfo) (string, error) {
	serviceDescriptions := make([]string, len(services))
	for i, service := range services {
		schemaStr, err := json.Marshal(service.Schema)
		if err != nil {
			return "", fmt.Errorf("failed to marshal service schema: %w", err)
		}
		serviceDescriptions[i] = fmt.Sprintf("Service ID: %s\nService Name: %s\nDescription: %s\nSchema: %s", service.ID, service.Name, service.Description, string(schemaStr))
	}

	actionStr := orchestration.Action.Content

	dataStr, err := json.Marshal(orchestration.Params)
	if err != nil {
		return "", fmt.Errorf("failed to marshal data: %w", err)
	}

	prompt := fmt.Sprintf(`You are an AI orchestrator tasked with planning the execution of services based on a user's action. A user's action contains PARAMS for the action to be executed, USE THEM. Your goal is to create an efficient, parallel execution plan that fulfills the user's request.

Available Services:
%s

User Action: %s

Action Params:
%s

Guidelines:
1. Each service described above contains input/output types and description. You must strictly adhere to these types and descriptions when using the services.
2. Each task in the plan should strictly use one of the available services. Follow the JSON conventions for each task.
3. Each task MUST have a unique ID, which is strictly increasing.
4. Inputs for tasks can either be constants or outputs from preceding tasks. In the latter case, use the format $taskId to denote the ID of the previous task whose output will be the input.
5. Ensure the plan maximizes parallelizability.
6. Only use the provided services.
	- If a query cannot be addressed using these, USE A "final" TASK TO SUGGEST THE NEXT STEPS.
		- The final task MUST have "final" as the task ID.
		- The final task DOES NOT require a service.
		- The final task input PARAM key should be "error" and the value should explain why the query cannot be addressed.   
		- NO OTHER TASKS ARE REQUIRED. 
7. Never explain the plan with comments.
8. Never introduce new services other than the ones provided.

Please generate a plan in the following JSON format:

{
  "tasks": [
    {
      "id": "task1",
      "service": "ServiceID",
      "input": {
        "param1": "value1",
        "param2": "$taskId"
      }
    },
    {
      "id": "task2",
      "service": "AnotherServiceID",
      "input": {
        "param1": "$task1"
      }
    }
  ],
  "parallel_groups": [
    ["task1", "task3"],
    ["task2", "task4"]
  ]
}

Ensure that the plan is efficient, maximizes parallelization, and accurately fulfills the user's action using the available services. If the action cannot be completed with the given services, explain why in a "final" task and suggest alternatives if possible.

Generate the execution plan:`, strings.Join(serviceDescriptions, "\n\n"), actionStr, string(dataStr))

	return prompt, nil
}

func (p *ControlPlane) decomposeAction(orchestration *Orchestration, services []*ServiceInfo) (*ServiceCallingPlan, error) {
	prompt, err := p.generateLLMPrompt(orchestration, services)
	if err != nil {
		return nil, fmt.Errorf("error generating LLM prompt for decomposing actions: %v", err)
	}

	log.Println("decomposeAction prompt:", prompt)

	client := openai.NewClient(p.openAIKey)
	resp, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model: openai.GPT4oLatest,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
	})

	if err != nil {
		return nil, fmt.Errorf("error calling OpenAI API: %v", err)
	}

	var result *ServiceCallingPlan
	sanitisedJSON := sanitizeJSONOutput(resp.Choices[0].Message.Content)
	log.Println("resp.Choices[0].Message.Content", sanitisedJSON)

	err = json.Unmarshal([]byte(sanitisedJSON), &result)
	if err != nil {
		return nil, fmt.Errorf("error parsing LLM response as JSON: %v", err)
	}

	result.ProjectID = orchestration.ProjectID

	return result, nil
}

func (p *ControlPlane) validateInput(services []*ServiceInfo, subTasks []*SubTask) error {
	serviceMap := make(map[string]*ServiceInfo)
	for _, service := range services {
		serviceMap[service.ID] = service
	}

	for _, subTask := range subTasks {
		service, ok := serviceMap[subTask.Service]
		if !ok {
			return fmt.Errorf("service %s not found for subtask %s", subTask.Service, subTask.ID)
		}

		for inputKey := range subTask.Input {
			if !service.Schema.InputIncludes(inputKey) {
				return fmt.Errorf("input %s not supported by service %s for subtask %s", inputKey, subTask.Service, subTask.ID)
			}
		}
	}

	return nil
}

func (p *ControlPlane) addServiceDetails(services []*ServiceInfo, subTasks []*SubTask) error {
	serviceMap := make(map[string]*ServiceInfo)
	for _, service := range services {
		serviceMap[service.ID] = service
	}

	for _, subTask := range subTasks {
		service, ok := serviceMap[subTask.Service]
		if !ok {
			return fmt.Errorf("service %s not found for subtask %s", subTask.Service, subTask.ID)
		}
		subTask.ServiceDetails = service.String()
	}

	return nil
}

func (p *ControlPlane) prepareOrchestration(orchestration *Orchestration) {
	services, err := p.discoverProjectServices(orchestration.ProjectID)
	if err != nil {
		log.Printf("Error discovering services: %v", err)
		orchestration.Status = Failed
		orchestration.Error = err.Error()
		return
	}

	callingPlan, err := p.decomposeAction(orchestration, services)
	if err != nil {
		log.Printf("Error decomposing action: %v", err)
		orchestration.Status = Failed
		orchestration.Error = fmt.Sprintf("Error decomposing action: %s", err.Error())
		return
	}

	if p.cannotExecuteAction(callingPlan.Tasks) {
		orchestration.Plan = callingPlan
		orchestration.Status = NotActionable
		orchestration.Error = string(callingPlan.Tasks[0].Input["error"])
		return
	}

	if err = p.validateInput(services, callingPlan.Tasks); err != nil {
		orchestration.Status = Failed
		orchestration.Error = fmt.Sprintf("Error validating plan input/output: %s", err.Error())
		return
	}

	if err := p.addServiceDetails(services, callingPlan.Tasks); err != nil {
		orchestration.Status = Failed
		orchestration.Error = fmt.Sprintf("Error adding service details to calling plan: %s", err.Error())
		return
	}

	orchestration.Plan = callingPlan
}

func (p *ControlPlane) cannotExecuteAction(subTasks []*SubTask) bool {
	return len(subTasks) == 1 && strings.EqualFold(subTasks[0].ID, "final")
}

func (p *ControlPlane) executeOrchestration(orchestration *Orchestration) {
	p.executePlan(orchestration)
	err := p.triggerWebhook(orchestration)
	if err != nil {
		log.Printf("Error triggering webhook: %v", err)
		return
	}
}

func (p *ControlPlane) executePlan(orchestration *Orchestration) {
	tm := NewOrchestrator()

	p.wsConnectionsMutex.RLock()
	tm.wsConns = p.wsConnections
	p.wsConnectionsMutex.RUnlock()

	for _, group := range orchestration.Plan.ParallelGroups {
		if err := tm.executeParallelGroup(group, orchestration); err != nil {
			orchestration.Status = Failed
			orchestration.Error = err.Error()
			log.Printf("Plan execution failed at group: %+v", group)
			return
		}
	}

	orchestration.Status = Completed
	if len(orchestration.Plan.Tasks) > 0 {
		lastTaskID := orchestration.Plan.Tasks[len(orchestration.Plan.Tasks)-1].ID
		if result, ok := tm.results[lastTaskID]; ok {
			orchestration.Results = []json.RawMessage{result}
		} else {
			log.Printf("Warning: Result for last task %s not found", lastTaskID)
		}
	} else {
		log.Printf("Warning: No tasks in the plan")
	}
	log.Printf("Plan execution completed successfully for orchestration %s", orchestration.ID)
}

func (p *ControlPlane) triggerWebhook(orchestration *Orchestration) error {
	project, ok := p.projects[orchestration.ProjectID]
	if !ok {
		log.Printf("Project %s not found", orchestration.ProjectID)
	}

	var resultInfo = struct {
		Results []json.RawMessage `json:"results"`
		Status  Status            `json:"status"`
		Error   string            `json:"error,omitempty"`
	}{
		Results: orchestration.Results,
		Status:  orchestration.Status,
		Error:   orchestration.Error,
	}

	payload, err := json.Marshal(resultInfo)
	if err != nil {
		return fmt.Errorf("failed to trigger webhook failed to marshal payload: %w", err)
	}

	log.Printf("Triggering webhook %s for project %s to send results:\n %s\n", project.Webhook, project.ID, payload)

	// Create a new request
	req, err := http.NewRequest("POST", project.Webhook, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "YourAppName/1.0")

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
			log.Printf("Failed to close response body: %v", err)
		}
	}(resp.Body)

	// Check the response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
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

func NewOrchestrator() *Orchestrator {
	return &Orchestrator{
		tasks:   make(map[string]*Task),
		results: make(map[string]json.RawMessage),
		wsConns: make(map[string]*ServiceConnection),
	}
}

func (o *Orchestrator) executeParallelGroup(group ParallelGroup, orchestration *Orchestration) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(group))

	for _, subTaskId := range group {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			subTask := o.findSubTaskByID(id, orchestration.Plan.Tasks)
			if subTask == nil {
				errChan <- fmt.Errorf("task %s not found", id)
				return
			}
			log.Printf("Found subTask %s with input %+v\n", subTask.ID, subTask.Input)
			if err := o.executeSubTask(subTask, orchestration.ID, orchestration.ProjectID); err != nil {
				subTask.Error = err.Error()
				subTask.Status = Failed
				errChan <- err
			} else {
				subTask.Status = Completed
			}
		}(subTaskId)
	}

	wg.Wait()
	close(errChan)

	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("group execution errors: %v", errors)
	}
	return nil
}

func (o *Orchestrator) findSubTaskByID(id string, tasks []*SubTask) *SubTask {
	for _, task := range tasks {
		if strings.EqualFold(task.ID, id) {
			return task
		}
	}
	return nil
}

func (o *Orchestrator) executeSubTask(subTask *SubTask, orchestrationID, projectID string) error {
	input, err := o.prepareInput(subTask.Input)
	if err != nil {
		return fmt.Errorf("failed to prepare input for task %s: %w", subTask.ID, err)
	}

	data, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("failed to marshal input for task %s: %w", subTask.ID, err)
	}

	task := &Task{
		ID:              subTask.ID,
		ServiceID:       subTask.Service,
		OrchestrationID: orchestrationID,
		ProjectID:       projectID,
		Input:           data,
		Status:          Pending,
	}

	return o.executeTask(task)
}

func (o *Orchestrator) prepareInput(input map[string]Source) (map[string]json.RawMessage, error) {
	o.resultsMu.RLock()
	defer o.resultsMu.RUnlock()

	prepared := make(map[string]json.RawMessage)
	for key, value := range input {
		if strings.HasPrefix(string(value), "$") {
			parts := strings.Split(strings.TrimPrefix(string(value), "$"), ".")
			taskID, sourceID := parts[0], parts[1]

			rawSource, ok := o.results[taskID]
			if !ok {
				return nil, fmt.Errorf("result for task %s not found", taskID)
			}

			var sourceData map[string]any
			err := json.Unmarshal(rawSource, &sourceData)
			if err != nil {
				return nil, err
			}

			sourceValue, ok := sourceData[sourceID]
			if !ok {
				return nil, fmt.Errorf("result for %s.%s not found", taskID, sourceID)
			}

			prepared[key] = json.RawMessage(fmt.Sprintf(`"%s"`, sourceValue))
		} else {
			prepared[key] = json.RawMessage(fmt.Sprintf(`"%s"`, value))
		}
	}

	return prepared, nil
}

func (o *Orchestrator) executeTask(task *Task) error {
	svcConn := o.getServiceConnection(task.ServiceID)
	if svcConn == nil {
		return fmt.Errorf("ServiceID %s has no connection", task.ServiceID)
	}

	task.Status = Processing
	resultChan := make(chan json.RawMessage, 1)
	errChan := make(chan error, 1)

	go o.handleTaskExecution(svcConn, task, resultChan, errChan)

	select {
	case result := <-resultChan:
		if len(result) < 1 {
			return fmt.Errorf("task %s execution failed with empty result [%s]", task.ID, string(result))
		}
		o.setResult(task.ID, result)
		return nil
	case err := <-errChan:
		return err
	case <-time.After(30 * time.Second):
		return fmt.Errorf("task execution timed out")
	}
}

func (o *Orchestrator) getServiceConnection(serviceID string) *ServiceConnection {
	o.wsConnsMu.RLock()
	defer o.wsConnsMu.RUnlock()
	return o.wsConns[serviceID]
}

func (o *Orchestrator) handleTaskExecution(svcConn *ServiceConnection, task *Task, resultChan chan<- json.RawMessage, errChan chan<- error) {
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
}

func (o *Orchestrator) setResult(taskID string, result json.RawMessage) {
	o.resultsMu.Lock()
	o.results[taskID] = result
	o.resultsMu.Unlock()
	atomic.AddInt32(&o.resultCount, 1)
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
