package main

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type ControlPlane struct {
	projects           map[string]*Project
	services           map[string][]*ServiceInfo
	orchestrationStore map[string]*Orchestration
	wsConnections      map[string]*ServiceConnection
	wsConnectionsMutex sync.RWMutex
	openAIKey          string
}

type Project struct {
	ID      string `json:"id"`
	APIKey  string `json:"apiKey"`
	Webhook string `json:"webhook"`
}

type ServiceConnection struct {
	Status ServiceStatus
	Conn   *websocket.Conn
}

type Orchestrator struct {
	tasks       map[string]*Task
	results     map[string]json.RawMessage
	tasksMu     sync.RWMutex
	resultsMu   sync.RWMutex
	wsConns     map[string]*ServiceConnection
	wsConnsMu   sync.RWMutex
	taskCount   int32
	resultCount int32
}

type Task struct {
	ID              string          `json:"id"`
	Input           json.RawMessage `json:"input"`
	ServiceID       string          `json:"-"`
	OrchestrationID string          `json:"-"`
	ProjectID       string          `json:"-"`
	Status          Status          `json:"-"`
}

// Source is either user input or the subtask Id of where the value is expected from
type Source string

type Spec struct {
	Type       string     `json:"type"`
	Properties Properties `json:"properties,omitempty"`
	Required   []string   `json:"required,omitempty"`
	Format     string     `json:"format,omitempty"`
	Minimum    int        `json:"minimum,omitempty"`
	Maximum    int        `json:"maximum,omitempty"`
}

type Properties map[string]Spec

type ServiceSchema struct {
	Input  Spec `json:"input"`
	Output Spec `json:"output"`
}

type ServiceInfo struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Schema      ServiceSchema `json:"schema"`
	Type        ServiceType   `json:"type"`
	ProjectID   string        `json:"-"`
}

type Orchestration struct {
	ID        string              `json:"id"`
	ProjectID string              `json:"-"`
	Action    Action              `json:"action"`
	Params    ActionParams        `json:"data"`
	Plan      *ServiceCallingPlan `json:"plan"`
	Results   []json.RawMessage   `json:"results"`
	Status    Status              `json:"status"`
	Error     string              `json:"error,omitempty"`
	Timestamp time.Time           `json:"timestamp"`
}

type Action struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

type ActionParams []ActionParam

type ActionParam struct {
	Field string `json:"field"`
	Value string `json:"value"`
}

// ServiceCallingPlan represents the execution plan for services and agents
type ServiceCallingPlan struct {
	ProjectID      string          `json:"-"`
	Tasks          []*SubTask      `json:"tasks"`
	ParallelGroups []ParallelGroup `json:"parallel_groups"`
}

type ParallelGroup []string

// SubTask represents a single task in the ServiceCallingPlan
type SubTask struct {
	ID             string            `json:"id"`
	Service        string            `json:"service"`
	ServiceDetails string            `json:"service_details"`
	Input          map[string]Source `json:"input"`
	Status         Status            `json:"status,omitempty"`
	Error          string            `json:"error,omitempty"`
}
