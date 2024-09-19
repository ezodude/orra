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

func NewOrchestrationPlatform() *OrchestrationPlatform {
	op := &OrchestrationPlatform{
		services:      make(map[string]*ServiceInfo),
		tasks:         make(chan *Task, 100),
		results:       make(chan *TaskResult, 100),
		wsConnections: make(map[string]chan *Task),
		projects:      make(map[string]*Project),
		taskStore:     make(map[string]*Task),
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

func (op *OrchestrationPlatform) executeTask(task *Task) {
	// Placeholder: Determine how to run the task
	serviceName := op.determineService(task)

	if taskChan, ok := op.wsConnections[serviceName]; ok {
		taskChan <- task
	} else {
		log.Printf("Service %s not connected", serviceName)
	}
}

func (op *OrchestrationPlatform) determineService(task *Task) string {
	// Placeholder: Implement logic to determine which service should handle the task
	return task.Service
}

func (op *OrchestrationPlatform) processResult(result *TaskResult) {
	task, ok := op.taskStore[result.TaskID]
	if !ok {
		log.Printf("Task %s not found", result.TaskID)
		return
	}

	// Placeholder: Implement result aggregation logic
	op.aggregateResult(task, result)

	// Check if this is the final result for the project
	if op.isProjectComplete(task.ProjectID) {
		op.sendWebhook(task.ProjectID)
	}
}

func (op *OrchestrationPlatform) aggregateResult(task *Task, _ *TaskResult) {
	// Placeholder: Implement result aggregation logic
	log.Printf("Aggregating result for task %s", task.ID)
}

// isProjectComplete accepts a projectID
func (op *OrchestrationPlatform) isProjectComplete(_ string) bool {
	// Placeholder: Implement logic to check if all tasks for a project are complete
	return true
}

func (op *OrchestrationPlatform) sendWebhook(projectID string) {
	project, ok := op.projects[projectID]
	if !ok {
		log.Printf("Project %s not found", projectID)
		return
	}

	// Placeholder: Implement webhook sending logic
	log.Printf("Sending webhook for project %s to %s", projectID, project.Webhook)
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
	service.ProjectId = project.ID
	service.Type = serviceType
	op.services[service.ID] = &service
	op.wsConnections[service.ID] = make(chan *Task, 100)

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

func (op *OrchestrationPlatform) SubmitTask(w http.ResponseWriter, r *http.Request) {
	var task Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	task.ID = uuid.New().String()
	task.Status = Pending

	op.taskStore[task.ID] = &task
	op.tasks <- &task

	if err := json.NewEncoder(w).Encode(map[string]string{"taskId": task.ID}); err != nil {
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
	taskChan, ok := op.wsConnections[serviceId]
	if !ok {
		log.Printf("Service %s not registered", serviceId)

		if err := conn.Close(); err != nil {
			log.Println(err)
			return
		}
		return
	}

	go op.writeLoop(conn, taskChan)
	go op.readLoop(conn, serviceId)
}

func (op *OrchestrationPlatform) writeLoop(conn *websocket.Conn, taskChan <-chan *Task) {
	for task := range taskChan {
		if err := conn.WriteJSON(task); err != nil {
			log.Printf("Failed to send task: %v", err)
			return
		}
	}
}

func (op *OrchestrationPlatform) readLoop(conn *websocket.Conn, serviceId string) {
	defer func(conn *websocket.Conn) {
		err := conn.Close()
		if err != nil {
			log.Println(err)
		}
	}(conn)

	for {
		var result TaskResult
		if err := conn.ReadJSON(&result); err != nil {
			log.Printf("Failed to read result: %v", err)
			delete(op.wsConnections, serviceId)
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
	r.HandleFunc("/task", APIKeyMiddleware(op.SubmitTask)).Methods("POST")
	r.HandleFunc("/register/service", APIKeyMiddleware(op.RegisterService)).Methods("POST")
	r.HandleFunc("/register/agent", APIKeyMiddleware(op.RegisterAgent)).Methods("POST")
	r.HandleFunc("/ws", op.HandleWebSocket)

	log.Println("Starting server on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
