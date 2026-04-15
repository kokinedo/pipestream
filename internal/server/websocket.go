package server

import (
	"sync"

	"github.com/kokinedo/pipestream/pkg/models"
)

// EventBus is a fan-out pub/sub for classified events.
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[uint64]chan *models.ClassifiedEvent
	nextID      uint64
}

// NewEventBus creates an EventBus.
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[uint64]chan *models.ClassifiedEvent),
	}
}

// Publish sends an event to all subscribers without blocking.
// If a subscriber's buffer is full, the event is dropped for that subscriber.
func (b *EventBus) Publish(event *models.ClassifiedEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, ch := range b.subscribers {
		select {
		case ch <- event:
		default:
			// Drop event for slow subscriber.
		}
	}
}

// Subscribe returns a channel that receives events and an unsubscribe function.
func (b *EventBus) Subscribe() (<-chan *models.ClassifiedEvent, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := b.nextID
	b.nextID++
	ch := make(chan *models.ClassifiedEvent, 100)
	b.subscribers[id] = ch

	unsub := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.subscribers, id)
		close(ch)
	}
	return ch, unsub
}
