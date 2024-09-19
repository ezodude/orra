package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for this example
	},
}

func (o *Orchestration) isComplete() bool {
	return true
}

func NewOrchestrationPlatform() *OrchestrationPlatform {
	op := &OrchestrationPlatform{
		tasks:              make(chan *Task, 100),
		results:            make(chan *TaskResult, 100),
		wsConnections:      make(map[string]*ServiceConnection),
		projects:           make(map[string]*Project),
		services:           make(map[string][]*ServiceInfo),
		orchestrationStore: make(map[string]*Orchestration),
		taskStore:          make(map[string]*Task),
	}
	go op.orchestrator()
	return op
}

func (op *OrchestrationPlatform) orchestrator() {
	for {
		select {
		case task := <-op.tasks:
			go op.executeTask(task)
		case result := <-op.results:
			op.processResult(result)
		}
	}
}

func (op *OrchestrationPlatform) determineServices(orchestration *Orchestration) []*ServiceInfo {
	// Placeholder: Implement logic to determine which service should handle the task
	return op.services[orchestration.ProjectID]
}

func (op *OrchestrationPlatform) executeOrchestration(orchestration *Orchestration) {
	// Placeholder: Determine how to run the task
	services := op.determineServices(orchestration)

	// Placeholder: Input added to any task based schema below SHOULD BE UPDATED FOR SPECIFIC TASKS
	//const serviceSchema =
	//{
	//  input:
	//  {
	//		type: 'object',
	//		fields: [ { name: 'customerId', type: 'string', format: 'uuid' } ],
	//    required: [ 'customerId' ]
	//  },
	//  output: {
	//    type: 'object',
	//    fields: [
	//      { name: 'id', type: 'string', format: 'uuid' },
	//      { name: 'name', type: 'string' },
	//      { name: 'balance', type: 'number', minimum: 0 }
	//    ]
	//  }
	//};

	for _, serviceInfo := range services {
		task := Task{
			ID:              uuid.New().String(),
			ServiceID:       serviceInfo.ID,
			OrchestrationID: orchestration.ID,
			ProjectID:       serviceInfo.ProjectID,
			Input:           []byte(`{"customerId": "12aaf63d-6331-4377-83d5-ff75a5d36dc6"}`),
			Status:          Pending,
		}
		op.taskStore[task.ID] = &task
		op.tasks <- &task
		orchestration.Status = Processing
	}
}

func (op *OrchestrationPlatform) executeTask(task *Task) {
	// Placeholder: Determine how to run the task
	if svcConn, ok := op.wsConnections[task.ServiceID]; ok {
		task.Status = Processing
		svcConn.TaskWorkChan <- task
	} else {
		log.Printf("ServiceID %s not connected", task.ServiceID)
	}
}

func (op *OrchestrationPlatform) processResult(result *TaskResult) {
	task, ok := op.taskStore[result.TaskID]
	if !ok {
		log.Printf("Task %s not found", result.TaskID)
		return
	}
	orchestration, ok := op.orchestrationStore[task.OrchestrationID]
	if !ok {
		log.Printf("Orchestration %s not found", task.OrchestrationID)
		return
	}
	task.Status = Completed
	orchestration.Status = Completed
	// Placeholder: Implement result aggregation logic
	op.aggregateResult(orchestration, result)

	// Check if this is the final result for the project
	if op.isOrchestrationComplete(orchestration) {
		op.sendWebhook(orchestration.ProjectID, orchestration.Results)
	}
}

func (op *OrchestrationPlatform) aggregateResult(orchestration *Orchestration, result *TaskResult) {
	// Placeholder: Implement result aggregation logic
	orchestration.Results = append(orchestration.Results, result.Result)
	log.Printf("Aggregating result for Orchestration %s and Task %s", orchestration.ID, result.TaskID)
}

// isOrchestrationComplete accepts an orchestration ID
func (op *OrchestrationPlatform) isOrchestrationComplete(orchestration *Orchestration) bool {
	// Placeholder: Implement logic to check if all tasks for a project are complete
	return orchestration.isComplete()
}

func (op *OrchestrationPlatform) sendWebhook(projectID string, results []json.RawMessage) {
	project, ok := op.projects[projectID]
	if !ok {
		log.Printf("Project %s not found", projectID)
		return
	}

	// Placeholder: Implement webhook sending logic
	log.Printf("Using webhook %s for project %s to send resulrs %s", projectID, project.Webhook, results)
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
		TaskWorkChan: make(chan *Task, 100),
		Status:       Disconnected,
	}

	if err := json.NewEncoder(w).Encode(map[string]any{
		"id":     service.ID,
		"name":   service.Name,
		"status": Registered,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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

	log.Println("Am I blocked?")
	go op.executeOrchestration(&orchestration)
	log.Println("I AM NOT BLOCKED!!!")

	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(orchestration); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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

	go op.writeLoop(serviceConn)
	go op.readLoop(serviceConn)
}

func (op *OrchestrationPlatform) writeLoop(serviceConn *ServiceConnection) {
	for task := range serviceConn.TaskWorkChan {
		if serviceConn.Status == Disconnected {
			// Queue the task or handle the disconnected state
			continue
		}

		if err := serviceConn.Conn.WriteJSON(task); err != nil {
			log.Printf("Failed to send task: %v", err)
			serviceConn.Status = Disconnected
			return
		}
	}
}

func (op *OrchestrationPlatform) readLoop(serviceConn *ServiceConnection) {
	defer func(sc *ServiceConnection) {
		err := sc.Conn.Close()
		if err != nil {
			log.Println(err)
		}
		serviceConn.Status = Disconnected
	}(serviceConn)

	for {
		var result TaskResult
		if err := serviceConn.Conn.ReadJSON(&result); err != nil {
			log.Printf("Failed to read result: %v", err)
			return
		}
		op.results <- &result
	}
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
	op := NewOrchestrationPlatform()

	r := mux.NewRouter()
	r.HandleFunc("/register/project", op.RegisterProject).Methods("POST")
	r.HandleFunc("/orchestrations", APIKeyMiddleware(op.OrchestrationsHandler)).Methods("POST")
	r.HandleFunc("/register/service", APIKeyMiddleware(op.RegisterService)).Methods("POST")
	r.HandleFunc("/register/agent", APIKeyMiddleware(op.RegisterAgent)).Methods("POST")
	r.HandleFunc("/ws", op.HandleWebSocket)

	log.Println("Starting server on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
