package main

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

type ControlPlane struct {
	projects             map[string]*Project
	services             map[string][]*ServiceInfo
	orchestrationStore   map[string]*Orchestration
	orchestrationStoreMu sync.RWMutex
	wsConnections        map[string]*ServiceConnection
	wsConnectionsMutex   sync.RWMutex
	LogManager           *LogManager
	logWorkers           map[string]map[string]context.CancelFunc
	workerMu             sync.RWMutex
	openAIKey            string
	Logger               zerolog.Logger
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

type OrchestrationState struct {
	ID             string
	ProjectID      string
	Plan           *ServiceCallingPlan
	CompletedTasks map[string]bool
	Status         Status
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Error          string
}

type LogEntry struct {
	Offset     uint64          `json:"offset"`
	Type       string          `json:"type"`
	ID         string          `json:"id"`
	Value      json.RawMessage `json:"value"`
	Timestamp  time.Time       `json:"timestamp"`
	ProducerID string          `json:"producer_id"`
	AttemptNum int             `json:"attempt_num"`
}

type LogManager struct {
	logs           map[string]*Log
	orchestrations map[string]*OrchestrationState
	mu             sync.RWMutex
	retention      time.Duration
	cleanupTicker  *time.Ticker
	webhookClient  *http.Client
	controlPlane   *ControlPlane
	Logger         zerolog.Logger
}

type Log struct {
	Entries       []LogEntry
	CurrentOffset uint64
	mu            sync.RWMutex
	lastAccessed  time.Time // For cleanup
}

type DependencyState map[string]json.RawMessage

type LogState struct {
	LastOffset      uint64
	Processed       map[string]bool
	DependencyState DependencyState
}

type LogWorker interface {
	Start(ctx context.Context, orchestrationID string)
	PollLog(ctx context.Context, logStream *Log, entriesChan chan<- LogEntry)
}

type ResultAggregator struct {
	Dependencies DependencyKeys
	LogManager   *LogManager
	logState     *LogState
	stateMu      sync.Mutex
}

type TaskWorker struct {
	ServiceID    string
	TaskID       string
	Dependencies DependencyKeys
	LogManager   *LogManager
	logState     *LogState
	stateMu      sync.Mutex
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
	taskZero  json.RawMessage
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

type DependencyKeys map[string]struct{}

// SubTask represents a single task in the ServiceCallingPlan
type SubTask struct {
	ID             string            `json:"id"`
	Service        string            `json:"service"`
	ServiceDetails string            `json:"service_details"`
	Input          map[string]Source `json:"input"`
	Status         Status            `json:"status,omitempty"`
	Error          string            `json:"error,omitempty"`
}
