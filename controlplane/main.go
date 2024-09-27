package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "github.com/joho/godotenv/autoload"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/sashabaranov/go-openai"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for this example
	},
}

func NewOrchestrationPlatform(openAIKey string) *OrchestrationPlatform {
	return &OrchestrationPlatform{
		wsConnections:      make(map[string]*ServiceConnection),
		projects:           make(map[string]*Project),
		services:           make(map[string][]*ServiceInfo),
		orchestrationStore: make(map[string]*Orchestration),
		openAIKey:          openAIKey,
	}
}

func (op *OrchestrationPlatform) discoverProjectServices(projectID string) ([]*ServiceInfo, error) {
	services, ok := op.services[projectID]
	if !ok {
		return nil, fmt.Errorf("no services found for project %s", projectID)
	}
	return services, nil
}

func (op *OrchestrationPlatform) decomposeAction(orchestration *Orchestration, services []*ServiceInfo) (*ServiceCallingPlan, error) {
	prompt, err := op.generateLLMPrompt(orchestration, services)
	if err != nil {
		return nil, fmt.Errorf("error generating LLM prompt for decomposing actions: %v", err)
	}

	log.Println("decomposeAction prompt:", prompt)

	client := openai.NewClient(op.openAIKey)
	resp, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model: openai.GPT4o20240806,
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

func (op *OrchestrationPlatform) validateInput(services []*ServiceInfo, subTasks []*SubTask) error {
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

func (op *OrchestrationPlatform) prepareOrchestration(orchestration *Orchestration) {
	services, err := op.discoverProjectServices(orchestration.ProjectID)
	if err != nil {
		log.Printf("Error discovering services: %v", err)
		orchestration.Status = Failed
		orchestration.Error = err.Error()
		return
	}

	callingPlan, err := op.decomposeAction(orchestration, services)
	if err != nil {
		log.Printf("Error decomposing action: %v", err)
		orchestration.Status = Failed
		orchestration.Error = fmt.Sprintf("Error decomposing action: %s", err.Error())
		return
	}

	if op.cannotExecuteAction(callingPlan.Tasks) {
		orchestration.Plan = callingPlan
		orchestration.Status = NotActionable
		orchestration.Error = string(callingPlan.Tasks[0].Input["error"])
		return
	}

	err = op.validateInput(services, callingPlan.Tasks)
	if err != nil {
		orchestration.Status = Failed
		orchestration.Error = fmt.Sprintf("Error validating plan input/output: %s", err.Error())
		return
	}

	orchestration.Plan = callingPlan
}

func (op *OrchestrationPlatform) executeOrchestration(orchestration *Orchestration) {
	op.executePlan(orchestration)
	op.triggerWebhook(orchestration)
}

func (op *OrchestrationPlatform) executePlan(orchestration *Orchestration) {
	tm := NewTaskManager()
	tm.wsConns = op.wsConnections

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

func (tm *TaskManager) executeParallelGroup(group ParallelGroup, orchestration *Orchestration) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(group))

	for _, subTaskId := range group {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			subTask := tm.findSubTaskByID(id, orchestration.Plan.Tasks)
			if subTask == nil {
				errChan <- fmt.Errorf("task %s not found", id)
				return
			}
			log.Printf("Found subTask %s with input %+v\n", subTask.ID, subTask.Input)
			if err := tm.executeSubTask(subTask, orchestration.ID, orchestration.ProjectID); err != nil {
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

func (tm *TaskManager) findSubTaskByID(id string, tasks []*SubTask) *SubTask {
	for _, task := range tasks {
		if strings.EqualFold(task.ID, id) {
			return task
		}
	}
	return nil
}

func (op *OrchestrationPlatform) triggerWebhook(orchestration *Orchestration) {
	project, ok := op.projects[orchestration.ProjectID]
	if !ok {
		log.Printf("Project %s not found", orchestration.ProjectID)
	}

	data, err := json.Marshal(orchestration)
	if err != nil {
		log.Printf("Failed to trigger webhook for projectID %s and orchestrationID %s", orchestration.ProjectID, orchestration.ID)
	}

	// Placeholder: Implement webhook sending logic
	log.Printf("Triggering webhook %s for project %s to send results:\n %s\n", project.Webhook, project.ID, data)
}

func (tm *TaskManager) executeSubTask(subTask *SubTask, orchestrationID, projectID string) error {
	input, err := tm.prepareInput(subTask.Input)
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

	return tm.executeTask(task)
}

func (tm *TaskManager) prepareInput(input map[string]Source) (map[string]json.RawMessage, error) {
	tm.resultsMu.RLock()
	defer tm.resultsMu.RUnlock()

	prepared := make(map[string]json.RawMessage)
	for key, source := range input {
		if strings.HasPrefix(string(source), "$") {
			taskID := strings.TrimPrefix(string(source), "$")
			result, ok := tm.results[taskID]
			if !ok {
				return nil, fmt.Errorf("result for task %s not found", taskID)
			}
			prepared[key] = result
		} else {
			prepared[key] = json.RawMessage(fmt.Sprintf(`"%s"`, source))
		}
	}

	return prepared, nil
}

func (tm *TaskManager) executeTask(task *Task) error {
	svcConn := tm.getServiceConnection(task.ServiceID)
	if svcConn == nil {
		return fmt.Errorf("ServiceID %s not connected", task.ServiceID)
	}

	task.Status = Processing
	resultChan := make(chan json.RawMessage, 1)
	errChan := make(chan error, 1)

	go tm.handleTaskExecution(svcConn, task, resultChan, errChan)

	select {
	case result := <-resultChan:
		tm.setResult(task.ID, result)
		return nil
	case err := <-errChan:
		return err
	case <-time.After(30 * time.Second):
		return fmt.Errorf("task execution timed out")
	}
}

func (tm *TaskManager) getServiceConnection(serviceID string) *ServiceConnection {
	tm.wsConnsMu.RLock()
	defer tm.wsConnsMu.RUnlock()
	return tm.wsConns[serviceID]
}

func (tm *TaskManager) handleTaskExecution(svcConn *ServiceConnection, task *Task, resultChan chan<- json.RawMessage, errChan chan<- error) {
	if err := svcConn.Conn.WriteJSON(task); err != nil {
		errChan <- fmt.Errorf("failed to send task: %w", err)
		return
	}

	var result TaskResult
	if err := svcConn.Conn.ReadJSON(&result); err != nil {
		errChan <- fmt.Errorf("failed to read result: %w", err)
		return
	}

	resultChan <- result.Result
}

func (tm *TaskManager) setResult(taskID string, result json.RawMessage) {
	tm.resultsMu.Lock()
	tm.results[taskID] = result
	tm.resultsMu.Unlock()
	atomic.AddInt32(&tm.resultCount, 1)
}

func (tm *TaskManager) GetResultCount() int {
	return int(atomic.LoadInt32(&tm.resultCount))
}

func (tm *TaskManager) GetTaskCount() int {
	return int(atomic.LoadInt32(&tm.taskCount))
}
func (op *OrchestrationPlatform) generateLLMPrompt(orchestration *Orchestration, services []*ServiceInfo) (string, error) {
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

func (op *OrchestrationPlatform) RegisterProject(w http.ResponseWriter, r *http.Request) {
	var project Project
	if err := json.NewDecoder(r.Body).Decode(&project); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	project.ID = uuid.New().String()
	project.APIKey = uuid.New().String()

	op.projects[project.ID] = &project

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(project); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (op *OrchestrationPlatform) RegisterServiceOrAgent(w http.ResponseWriter, r *http.Request, serviceType ServiceType) {
	apiKey := r.Context().Value("api_key").(string)
	project, err := op.GetProjectByApiKey(apiKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var service ServiceInfo
	if err := json.NewDecoder(r.Body).Decode(&service); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	service.ID = uuid.New().String()
	service.ProjectID = project.ID
	service.Type = serviceType

	// need a better to add services that avoid duplicating service registration
	op.services[project.ID] = append(op.services[project.ID], &service)
	op.wsConnections[service.ID] = &ServiceConnection{
		Status: Disconnected,
	}

	if err := json.NewEncoder(w).Encode(map[string]any{
		"id":     service.ID,
		"name":   service.Name,
		"status": Registered,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (op *OrchestrationPlatform) RegisterService(w http.ResponseWriter, r *http.Request) {
	op.RegisterServiceOrAgent(w, r, Service)
}

func (op *OrchestrationPlatform) RegisterAgent(w http.ResponseWriter, r *http.Request) {
	op.RegisterServiceOrAgent(w, r, Agent)
}

func (op *OrchestrationPlatform) OrchestrationsHandler(w http.ResponseWriter, r *http.Request) {
	apiKey := r.Context().Value("api_key").(string)
	project, err := op.GetProjectByApiKey(apiKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var orchestration Orchestration
	if err := json.NewDecoder(r.Body).Decode(&orchestration); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	orchestration.ID = uuid.New().String()
	orchestration.Status = Pending
	orchestration.ProjectID = project.ID

	op.orchestrationStore[orchestration.ID] = &orchestration
	op.prepareOrchestration(&orchestration)

	if orchestration.Status == NotActionable || orchestration.Status == Failed {
		w.WriteHeader(http.StatusUnprocessableEntity)
	} else {
		go op.executeOrchestration(&orchestration)
		w.WriteHeader(http.StatusAccepted)
	}

	data, err := json.Marshal(orchestration)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err = w.Write(data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (op *OrchestrationPlatform) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	serviceId := r.URL.Query().Get("serviceId")
	op.mu.Lock()
	serviceConn, ok := op.wsConnections[serviceId]
	if !ok {
		log.Printf("ServiceID %s not registered", serviceId)
		if err := conn.Close(); err != nil {
			log.Printf("Error closing connection as after discovering ServiceID %s not registered\n", serviceId)
			return
		}
		return
	}
	serviceConn.Conn = conn
	serviceConn.Status = Connected

	op.mu.Unlock()
}

func APIKeyMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header is missing", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
			return
		}

		apiKey := parts[1]

		// Store the API key in the request context
		ctx := context.WithValue(r.Context(), "api_key", apiKey)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	}
}

func main() {
	cfg, err := Load()
	if err != nil {
		panic(err)
	}

	op := NewOrchestrationPlatform(cfg.OpenApiKey)

	r := mux.NewRouter()
	r.HandleFunc("/register/project", op.RegisterProject).Methods("POST")
	r.HandleFunc("/register/service", APIKeyMiddleware(op.RegisterService)).Methods("POST")
	r.HandleFunc("/orchestrations", APIKeyMiddleware(op.OrchestrationsHandler)).Methods("POST")
	r.HandleFunc("/register/agent", APIKeyMiddleware(op.RegisterAgent)).Methods("POST")
	r.HandleFunc("/ws", op.HandleWebSocket)

	log.Printf("Starting server on :%d\n", cfg.Port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", cfg.Port), r))
}
