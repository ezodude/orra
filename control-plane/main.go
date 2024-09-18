package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

type OrchestrationPlatform struct {
	services      map[string]*ServiceInfo
	tasks         chan *Task
	results       chan *TaskResult
	wsConnections map[string]chan *Task
	projects      map[string]*Project
	taskStore     map[string]*Task
}

type ServiceInfo struct {
	Name         string          `json:"name"`
	InputSchema  json.RawMessage `json:"inputSchema"`
	OutputSchema json.RawMessage `json:"outputSchema"`
}

type Task struct {
	ID        string          `json:"id"`
	Service   string          `json:"service"`
	Input     json.RawMessage `json:"input"`
	Status    string          `json:"status"`
	ProjectID string          `json:"projectId"`
}

type TaskResult struct {
	TaskID string          `json:"taskId"`
	Result json.RawMessage `json:"result"`
}

type Project struct {
	ID      string `json:"id"`
	APIKey  string `json:"apiKey"`
	Webhook string `json:"webhook"`
}

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

func (op *OrchestrationPlatform) RegisterService(w http.ResponseWriter, r *http.Request) {
	var service ServiceInfo
	if err := json.NewDecoder(r.Body).Decode(&service); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	op.services[service.Name] = &service
	op.wsConnections[service.Name] = make(chan *Task, 100)

	if err := json.NewEncoder(w).Encode(map[string]string{"status": "registered"}); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (op *OrchestrationPlatform) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	serviceName := r.URL.Query().Get("service")
	taskChan, ok := op.wsConnections[serviceName]
	if !ok {
		log.Printf("Service %s not registered", serviceName)

		if err := conn.Close(); err != nil {
			log.Println(err)
			return
		}
		return
	}

	go op.writeLoop(conn, taskChan)
	go op.readLoop(conn, serviceName)
}

func (op *OrchestrationPlatform) writeLoop(conn *websocket.Conn, taskChan <-chan *Task) {
	for task := range taskChan {
		if err := conn.WriteJSON(task); err != nil {
			log.Printf("Failed to send task: %v", err)
			return
		}
	}
}

func (op *OrchestrationPlatform) readLoop(conn *websocket.Conn, serviceName string) {
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
			delete(op.wsConnections, serviceName)
			return
		}
		op.results <- &result
	}
}

func (op *OrchestrationPlatform) SubmitTask(w http.ResponseWriter, r *http.Request) {
	var task Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	task.ID = uuid.New().String()
	task.Status = "pending"

	op.taskStore[task.ID] = &task
	op.tasks <- &task

	if err := json.NewEncoder(w).Encode(map[string]string{"taskId": task.ID}); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusAccepted)
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

func main() {
	op := NewOrchestrationPlatform()

	r := mux.NewRouter()
	r.HandleFunc("/register/service", op.RegisterService).Methods("POST")
	r.HandleFunc("/register/project", op.RegisterProject).Methods("POST")
	r.HandleFunc("/ws", op.HandleWebSocket)
	r.HandleFunc("/task", op.SubmitTask).Methods("POST")

	log.Println("Starting server on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
