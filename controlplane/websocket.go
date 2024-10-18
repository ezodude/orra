package main

import (
	"container/list"
	"encoding/json"
	"fmt"
	"time"

	"github.com/olahol/melody"
	"github.com/rs/zerolog"
)

func NewWebSocketManager(logger zerolog.Logger) *WebSocketManager {
	m := melody.New()
	m.Config.ConcurrentMessageHandling = true
	m.Config.WriteWait = WSWriteTimeOut

	return &WebSocketManager{
		melody:            m,
		logger:            logger,
		connMap:           make(map[string]*melody.Session),
		taskCallbacks:     make(map[string]WebSocketCallback),
		messageQueues:     make(map[string]*WebSocketMessageQueue),
		messageExpiration: time.Hour * 24, // Keep messages for 24 hours
		pingInterval:      m.Config.PingPeriod,
		pongWait:          m.Config.PongWait,
	}
}

func (wsm *WebSocketManager) HandleConnection(serviceID string, s *melody.Session) {
	s.Set("serviceID", serviceID)
	s.Set("lastPong", time.Now())

	wsm.connMu.Lock()
	wsm.connMap[serviceID] = s
	wsm.connMu.Unlock()

	go wsm.pingRoutine(s)
	wsm.sendQueuedMessages(serviceID, s)

	wsm.logger.Info().Str("serviceID", serviceID).Msg("New WebSocket connection established")
}

func (wsm *WebSocketManager) sendQueuedMessages(serviceID string, s *melody.Session) {
	wsm.messageQueuesMu.RLock()
	queue, exists := wsm.messageQueues[serviceID]
	wsm.messageQueuesMu.RUnlock()

	if !exists {
		return
	}

	queue.mu.Lock()
	defer queue.mu.Unlock()

	for e := queue.Front(); e != nil; e = e.Next() {
		msg := e.Value.(*WebSocketQueuedMessage)
		if time.Since(msg.Time) > wsm.messageExpiration {
			queue.Remove(e)
			continue
		}

		wsm.logger.Debug().
			Fields(map[string]any{"serviceID": serviceID, "task": string(msg.Message)}).
			Msg("Queueing up message for disconnected Service")

		if err := s.Write(msg.Message); err != nil {
			wsm.logger.Error().Err(err).Str("serviceID", serviceID).Msg("Failed to send queued message")
			return
		}
		queue.Remove(e)
	}
}

func (wsm *WebSocketManager) HandleDisconnection(serviceID string) {
	wsm.connMu.Lock()
	delete(wsm.connMap, serviceID)
	wsm.connMu.Unlock()

	wsm.logger.Info().Str("serviceID", serviceID).Msg("WebSocket connection closed")
}

func (wsm *WebSocketManager) HandleMessage(s *melody.Session, msg []byte) {
	var message struct {
		Type        string          `json:"type"`
		TaskID      string          `json:"taskId"`
		ExecutionID string          `json:"executionId"`
		Result      json.RawMessage `json:"result,omitempty"`
		Error       string          `json:"error,omitempty"`
	}

	if err := json.Unmarshal(msg, &message); err != nil {
		wsm.logger.Error().Err(err).Msg("Failed to unmarshal WebSocket message")
		return
	}

	switch message.Type {
	case WSPong:
		s.Set("lastPong", time.Now())
	case "task_result":
		wsm.handleTaskResult(message)
	default:
		wsm.logger.Warn().Str("type", message.Type).Msg("Received unknown message type")
	}
}

func (wsm *WebSocketManager) handleTaskResult(message struct {
	Type        string          `json:"type"`
	TaskID      string          `json:"taskId"`
	ExecutionID string          `json:"executionId"`
	Result      json.RawMessage `json:"result,omitempty"`
	Error       string          `json:"error,omitempty"`
}) {
	wsm.callbacksMu.RLock()
	callback, exists := wsm.taskCallbacks[message.ExecutionID]
	wsm.callbacksMu.RUnlock()

	if !exists {
		wsm.logger.Error().Str("taskID", message.TaskID).Msg("No callback registered for task")
		return
	}

	if message.Error != "" {
		callback(nil, fmt.Errorf(message.Error))
	} else {
		callback(message.Result, nil)
	}

	wsm.UnregisterTaskCallback(message.ExecutionID)
}

func (wsm *WebSocketManager) SendTask(serviceID string, task *Task) error {
	wsm.connMu.RLock()
	session, connected := wsm.connMap[serviceID]
	wsm.connMu.RUnlock()

	message := struct {
		ID          string          `json:"id"`
		ExecutionID string          `json:"executionId"`
		Input       json.RawMessage `json:"input"`
	}{
		ID:          task.ID,
		ExecutionID: task.ExecutionID,
		Input:       task.Input,
	}

	jsonMessage, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to convert message to JSON for service %s: %w", serviceID, err)
	}

	if !connected {
		wsm.logger.Debug().
			Fields(map[string]any{"serviceID": serviceID, "taskID": task.ID}).
			Msg("Queueing up message for disconnected Service")
		wsm.QueueMessage(serviceID, jsonMessage)
		return nil
	}

	return session.Write(jsonMessage)
}

func (wsm *WebSocketManager) QueueMessage(serviceID string, message []byte) {
	wsm.messageQueuesMu.Lock()
	queue, exists := wsm.messageQueues[serviceID]
	if !exists {
		queue = &WebSocketMessageQueue{List: list.New()}
		wsm.messageQueues[serviceID] = queue
	}
	wsm.messageQueuesMu.Unlock()

	queue.mu.Lock()
	defer queue.mu.Unlock()
	if queue.Len() >= MaxQueueSize {
		wsm.logger.Warn().Str("serviceID", serviceID).Msg("Message queue full, dropping oldest message")
		queue.Remove(queue.Front())
	}
	queue.PushBack(&WebSocketQueuedMessage{Message: message, Time: time.Now()})
}

func (wsm *WebSocketManager) RegisterTaskCallback(executionID string, callback WebSocketCallback) {
	wsm.callbacksMu.Lock()
	defer wsm.callbacksMu.Unlock()
	wsm.taskCallbacks[executionID] = callback
}

func (wsm *WebSocketManager) UnregisterTaskCallback(executionID string) {
	wsm.callbacksMu.Lock()
	defer wsm.callbacksMu.Unlock()
	delete(wsm.taskCallbacks, executionID)
}

func (wsm *WebSocketManager) pingRoutine(s *melody.Session) {
	ticker := time.NewTicker(wsm.pingInterval)
	defer ticker.Stop()

	for {
		<-ticker.C
		if err := s.Write([]byte(WSPing)); err != nil {
			wsm.logger.Warn().Err(err).Msg("Failed to send ping, closing connection")
			err := s.Close()
			if err != nil {
				wsm.logger.Warn().Err(err).Msg("Failed to close connection")
				return
			}
			return
		}

		lastPong, ok := s.Get("lastPong")
		if !ok {
			wsm.logger.Warn().Msg("Missing Pong, closing connection")
			err := s.Close()
			if err != nil {
				wsm.logger.Warn().Err(err).Msg("Failed to close connection")
				return
			}
			return
		}

		if time.Since(lastPong.(time.Time)) > wsm.pongWait {
			wsm.logger.Warn().Msg("Pong timeout, closing connection")
			err := s.Close()
			if err != nil {
				wsm.logger.Warn().Err(err).Msg("Failed to close connection")
				return
			}
			return
		}
	}
}

// CleanupExpiredMessages cleans up expired messages
func (wsm *WebSocketManager) CleanupExpiredMessages() {
	wsm.messageQueuesMu.RLock()
	defer wsm.messageQueuesMu.RUnlock()

	for serviceID, queue := range wsm.messageQueues {
		queue.mu.Lock()
		var next *list.Element
		for e := queue.Front(); e != nil; e = next {
			next = e.Next()
			msg := e.Value.(*WebSocketQueuedMessage)
			if time.Since(msg.Time) > wsm.messageExpiration {
				queue.Remove(e)
			}
		}
		queue.mu.Unlock()

		wsm.logger.Debug().Str("serviceID", serviceID).Int("queueLength", queue.Len()).Msg("Cleaned up expired messages")
	}
}
