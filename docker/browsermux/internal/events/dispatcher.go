package events

import (
	"sync"
)

// Dispatcher handles event registration and distribution
type Dispatcher interface {
	Register(eventType EventType, handler EventHandler)
	Unregister(eventType EventType, handler EventHandler)
	Dispatch(event Event)
}

// defaultDispatcher is the default implementation of Dispatcher
type defaultDispatcher struct {
	handlers map[EventType][]EventHandler
	mu       sync.RWMutex
}

// NewDispatcher creates a new event dispatcher
func NewDispatcher() Dispatcher {
	return &defaultDispatcher{
		handlers: make(map[EventType][]EventHandler),
	}
}

// Register adds an event handler for a specific event type
func (d *defaultDispatcher) Register(eventType EventType, handler EventHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	if _, ok := d.handlers[eventType]; !ok {
		d.handlers[eventType] = []EventHandler{}
	}
	
	d.handlers[eventType] = append(d.handlers[eventType], handler)
}

// Unregister removes an event handler for a specific event type
func (d *defaultDispatcher) Unregister(eventType EventType, handler EventHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	if handlers, ok := d.handlers[eventType]; ok {
		for i, h := range handlers {
			// Compare function pointers (this is approximate)
			if &h == &handler {
				// Remove the handler by slicing
				d.handlers[eventType] = append(handlers[:i], handlers[i+1:]...)
				break
			}
		}
	}
}

// Dispatch sends an event to all registered handlers
func (d *defaultDispatcher) Dispatch(event Event) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	
	// Dispatch to specific event type handlers
	if handlers, ok := d.handlers[event.Type]; ok {
		for _, handler := range handlers {
			go handler(event) // Non-blocking dispatch
		}
	}
	
	// Dispatch to wildcard handlers (if any)
	if handlers, ok := d.handlers["*"]; ok {
		for _, handler := range handlers {
			go handler(event) // Non-blocking dispatch
		}
	}
}