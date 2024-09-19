package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

type OrchestrationPlatform struct {
	services      map[string]*ServiceInfo
	tasks         chan *Task
	results       chan *TaskResult
	wsConnections map[string]chan *Task
	projects      map[string]*Project
	taskStore     map[string]*Task
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

type Status int

const (
	Registered Status = iota
	Pending    Status = iota
)

func (s *Status) String() string {
	return [...]string{"registered", "pending"}[*s]
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

func (ss *ServiceType) String() string {
	return [...]string{"agent", "service"}[*ss]
}

func (ss *ServiceType) MarshalJSON() ([]byte, error) {
	return json.Marshal(ss.String())
}

func (ss *ServiceType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "agent":
		*ss = Agent
	case "service":
		*ss = Service
	default:
		return fmt.Errorf("invalid ServiceType: %s", s)
	}
	return nil
}

type ServiceInfo struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema"`
	Type        ServiceType     `json:"type"`
	ProjectId   string          `json:"-"`
}

type Task struct {
	ID        string          `json:"id"`
	Service   string          `json:"service"`
	Input     json.RawMessage `json:"input"`
	Status    Status          `json:"status"`
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
