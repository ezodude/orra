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
	m.Config.MaxMessageSize = WSMaxMessageBytes

	return &WebSocketManager{
		melody:            m,
		logger:            logger,
		connMap:           make(map[string]*melody.Session),
		messageQueues:     make(map[string]*WebSocketMessageQueue),
		messageExpiration: time.Hour * 24, // Keep messages for 24 hours
		pingInterval:      m.Config.PingPeriod,
		pongWait:          m.Config.PongWait,
		serviceHealth:     make(map[string]bool),
	}
}

func (wsm *WebSocketManager) HandleConnection(serviceID string, serviceName string, s *melody.Session) {
	s.Set("serviceID", serviceID)
	s.Set("lastPong", time.Now().UTC())

	wsm.connMu.Lock()
	wsm.connMap[serviceID] = s
	wsm.connMu.Unlock()

	go wsm.pingRoutine(s)
	wsm.sendQueuedMessages(serviceID, s)

	wsm.logger.Info().
		Str("serviceID", serviceID).
		Str("serviceName", serviceName).
		Msg("New WebSocket connection established")
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
			Msg("Sending queued message")

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

	wsm.logger.Info().Str("ServiceID", serviceID).Msg("WebSocket connection closed")
}

func (wsm *WebSocketManager) HandleMessage(s *melody.Session, msg []byte, fn ServiceFinder) {
	var messageWrapper struct {
		ID      string          `json:"id"`
		Payload json.RawMessage `json:"payload"`
	}

	var messagePayload TaskResult

	if err := json.Unmarshal(msg, &messageWrapper); err != nil {
		wsm.logger.Error().Err(err).Msg("Failed to unmarshal wrapped WebSocket messageWrapper")
		return
	}

	if err := json.Unmarshal(messageWrapper.Payload, &messagePayload); err != nil {
		wsm.logger.Error().Err(err).Msg("Failed to unmarshal WebSocket messageWrapper payload")
		return
	}

	switch messagePayload.Type {
	case WSPong:
		s.Set("lastPong", time.Now().UTC())
	case "task_status":
		wsm.logger.
			Info().
			Str("IdempotencyKey", string(messagePayload.IdempotencyKey)).
			Str("ServiceID", messagePayload.ServiceID).
			Str("TaskID", messagePayload.TaskID).
			Str("ExecutionID", messagePayload.ExecutionID).
			Msgf("Task status: %s", messagePayload.Status)
	case "task_result":
		wsm.handleTaskResult(messagePayload, fn)
	default:
		wsm.logger.Warn().Str("type", messagePayload.Type).Msg("Received unknown messageWrapper type")
	}

	if err := wsm.acknowledgeMessageReceived(s, messageWrapper.ID); err != nil {
		wsm.logger.Error().Err(err).Msg("Failed to handle messageWrapper acknowledgement")
		return
	}
}

func (wsm *WebSocketManager) acknowledgeMessageReceived(s *melody.Session, id string) error {
	if isPong := id == WSPong; isPong {
		return nil
	}

	ack := struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	}{
		Type: "ACK",
		ID:   id,
	}

	acknowledgement, err := json.Marshal(ack)
	if err != nil {
		return fmt.Errorf("failed to marshal acknowledgement: %w", err)
	}

	if err := s.Write(acknowledgement); err != nil {
		wsm.logger.Error().Err(err).Msg("Failed to send ACK")
		return fmt.Errorf("failed to send acknowledgement of receipt: %w", err)
	}

	return nil
}

func (wsm *WebSocketManager) handleTaskResult(message TaskResult, fn ServiceFinder) {
	service, err := fn(message.ServiceID)
	if err != nil {
		wsm.logger.Error().
			Err(err).
			Str("serviceID", message.ServiceID).
			Msg("Failed to get service when handling task result")
		return
	}

	service.IdempotencyStore.UpdateExecutionResult(
		message.IdempotencyKey,
		message.Result,
		parseError(message.Error),
	)
}

func parseError(errStr string) error {
	if errStr == "" {
		return nil
	}
	return fmt.Errorf(errStr)
}

func (wsm *WebSocketManager) SendTask(serviceID string, task *Task) error {
	wsm.connMu.RLock()
	session, connected := wsm.connMap[serviceID]
	wsm.connMu.RUnlock()

	message, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to convert message to JSON for service %s: %w", serviceID, err)
	}

	if !connected {
		//wsm.logger.Debug().
		//	Fields(map[string]any{"serviceID": serviceID, "taskID": task.ID}).
		//	Msg("Queueing up message for disconnected Service")
		//wsm.QueueMessage(serviceID, message)
		//wsm.healthCallback(serviceID, false)
		return nil
	}

	return session.Write(message)
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
	queue.PushBack(&WebSocketQueuedMessage{Message: message, Time: time.Now().UTC()})
}

func (wsm *WebSocketManager) pingRoutine(s *melody.Session) {
	ticker := time.NewTicker(wsm.pingInterval)
	defer ticker.Stop()

	for {
		serviceID, _ := s.Get("serviceID")

		<-ticker.C
		if s.IsClosed() {
			wsm.UpdateServiceHealth(serviceID.(string), false)

			wsm.logger.Info().
				Str("ServiceID", serviceID.(string)).
				Msg("PING/PONG no longer required for closed connection")

			return
		}

		if err := s.Write([]byte(WSPing)); err != nil {
			wsm.logger.Warn().
				Str("ServiceID", serviceID.(string)).
				Err(err).
				Msg("Failed to send ping, closing connection")

			wsm.UpdateServiceHealth(serviceID.(string), false)

			if err := s.Close(); err != nil {
				return
			}
			return
		}

		// Check for pong response
		lastPong, ok := s.Get("lastPong")
		if !ok || time.Since(lastPong.(time.Time)) > wsm.pongWait {
			wsm.logger.Warn().
				Str("ServiceID", serviceID.(string)).
				Msg("Pong timeout, closing connection")

			wsm.UpdateServiceHealth(serviceID.(string), false)

			if err := s.Close(); err != nil {
				return
			}

			return
		}
		wsm.UpdateServiceHealth(serviceID.(string), true)
	}
}

func (wsm *WebSocketManager) RegisterHealthCallback(callback ServiceHealthCallback) {
	wsm.healthCallbackMu.Lock()
	defer wsm.healthCallbackMu.Unlock()
	wsm.healthCallback = callback
}

func (wsm *WebSocketManager) UpdateServiceHealth(serviceID string, isHealthy bool) {
	wsm.healthMu.Lock()
	wsm.serviceHealth[serviceID] = isHealthy
	wsm.healthMu.Unlock()

	wsm.healthCallbackMu.RLock()
	wsm.healthCallback(serviceID, isHealthy)
	wsm.healthCallbackMu.RUnlock()
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
