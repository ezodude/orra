package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type ServiceConnection struct {
	Status ServiceStatus
	Conn   *websocket.Conn
}

type ServiceStatus int

const (
	Disconnected ServiceStatus = iota
	Connected
)

type OrchestrationPlatform struct {
	tasks              chan *Task
	results            chan *TaskResult
	wsConnections      map[string]*ServiceConnection
	projects           map[string]*Project
	services           map[string][]*ServiceInfo
	orchestrationStore map[string]*Orchestration
	openAIKey          string
	mu                 sync.Mutex
}

func (op *OrchestrationPlatform) GetProjectByApiKey(key string) (*Project, error) {
	apiKeyToProjectID := make(map[string]string)
	for id, project := range op.projects {
		apiKeyToProjectID[project.APIKey] = id
	}

	if projectID, exists := apiKeyToProjectID[key]; exists {
		return op.projects[projectID], nil
	} else {
		return nil, fmt.Errorf("no project found with the given API key: %s", key)
	}
}

func (op *OrchestrationPlatform) cannotExecuteAction(subTasks []*SubTask) bool {
	return len(subTasks) == 1 && strings.EqualFold(subTasks[0].ID, "final")
}

type Status int

const (
	Registered    Status = iota
	Pending       Status = iota
	Processing    Status = iota
	Completed     Status = iota
	Failed        Status = iota
	NotActionable Status = iota
)

func (s *Status) String() string {
	return [...]string{"registered", "pending", "processing", "completed", "failed", "not-actionable"}[*s]
}

func (s *Status) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s *Status) UnmarshalJSON(data []byte) error {
	var val string
	if err := json.Unmarshal(data, &val); err != nil {
		return err
	}
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "registered":
		*s = Registered
	case "pending":
		*s = Pending
	case "processing":
		*s = Processing
	case "completed":
		*s = Completed
	case "failed":
		*s = Failed
	case "not-actionable":
		*s = NotActionable
	default:
		return fmt.Errorf("invalid Status: %+v", s)
	}
	return nil
}

type ServiceType int

const (
	Agent ServiceType = iota
	Service
)

func (st *ServiceType) String() string {
	return [...]string{"agent", "service"}[*st]
}

func (st *ServiceType) MarshalJSON() ([]byte, error) {
	return json.Marshal(st.String())
}

func (st *ServiceType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "agent":
		*st = Agent
	case "service":
		*st = Service
	default:
		return fmt.Errorf("invalid ServiceType: %s", s)
	}
	return nil
}

type ParallelGroup []string

// ServiceCallingPlan represents the execution plan for services and agents
type ServiceCallingPlan struct {
	ProjectID      string          `json:"-"`
	Tasks          []*SubTask      `json:"tasks"`
	ParallelGroups []ParallelGroup `json:"parallel_groups"`
}

// Source is either user input or the subtask Id of where the value is expected from
type Source string

// SubTask represents a single task in the ServiceCallingPlan
type SubTask struct {
	ID      string            `json:"id"`
	Service string            `json:"service"`
	Input   map[string]Source `json:"input"`
	Status  Status            `json:"status,omitempty"`
	Error   string            `json:"error,omitempty"`
}

type Properties map[string]Spec

type Spec struct {
	Type       string     `json:"type"`
	Properties Properties `json:"properties,omitempty"`
	Required   []string   `json:"required,omitempty"`
	Format     string     `json:"format,omitempty"`
	Minimum    int        `json:"minimum,omitempty"`
	Maximum    int        `json:"maximum,omitempty"`
}

type ServiceSchema struct {
	Input  Spec `json:"input"`
	Output Spec `json:"output"`
}

func (s ServiceSchema) InputToString() (string, error) {
	return s.Input.String()
}

func (s ServiceSchema) OutputToString() (string, error) {
	return s.Output.String()
}

func (s ServiceSchema) InputIncludes(src string) bool {
	return s.Input.IncludesProp(src)
}

func (s ServiceSchema) OutputIncludes(src string) bool {
	return s.Output.IncludesProp(src)
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

type ServiceInfo struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Schema      ServiceSchema `json:"schema"`
	Type        ServiceType   `json:"type"`
	ProjectID   string        `json:"-"`
}

type Action struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

type ActionParam struct {
	Field string `json:"field"`
	Value string `json:"value"`
}

type ActionParams []ActionParam

func (a ActionParams) String() (string, error) {
	data, err := json.Marshal(a)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

type Orchestration struct {
	ID        string              `json:"id"`
	ProjectID string              `json:"-"`
	Action    Action              `json:"action"`
	Params    ActionParams        `json:"data"`
	Plan      *ServiceCallingPlan `json:"plan"`
	Results   []json.RawMessage   `json:"results"`
	Status    Status              `json:"status"`
	Error     string              `json:"error"`
	Timestamp time.Time           `json:"timestamp"`
}

type Task struct {
	ID              string          `json:"id"`
	ServiceID       string          `json:"-"`
	OrchestrationID string          `json:"-"`
	ProjectID       string          `json:"-"`
	Input           json.RawMessage `json:"input"`
	Status          Status          `json:"-"`
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

type TaskManager struct {
	tasks       map[string]*Task
	results     map[string]json.RawMessage
	tasksMu     sync.RWMutex
	resultsMu   sync.RWMutex
	wsConns     map[string]*ServiceConnection
	wsConnsMu   sync.RWMutex
	taskCount   int32
	resultCount int32
}

func NewTaskManager() *TaskManager {
	return &TaskManager{
		tasks:   make(map[string]*Task),
		results: make(map[string]json.RawMessage),
		wsConns: make(map[string]*ServiceConnection),
	}
}
