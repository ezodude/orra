package main

import (
	"sync"
)

type InMemoryBroker struct {
	subscribers map[string][]func(message interface{})
	mu          sync.RWMutex
}

func NewInMemoryBroker() *InMemoryBroker {
	return &InMemoryBroker{
		subscribers: make(map[string][]func(message interface{})),
	}
}

func (b *InMemoryBroker) Publish(topic string, message interface{}) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, handler := range b.subscribers[topic] {
		go handler(message)
	}
	return nil
}

func (b *InMemoryBroker) Subscribe(topic string, handler func(message interface{})) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.subscribers[topic] = append(b.subscribers[topic], handler)
	return nil
}

func (b *InMemoryBroker) Unsubscribe(topic string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.subscribers, topic)
	return nil
}
