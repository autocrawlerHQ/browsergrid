package events

import (
	"fmt"
	"sync"
)

type Dispatcher interface {
	Register(eventType EventType, handler EventHandler) HandlerID
	Unregister(eventType EventType, handlerID HandlerID)
	Dispatch(event Event)
}

type HandlerID string

type handlerEntry struct {
	id      HandlerID
	handler EventHandler
}

type defaultDispatcher struct {
	handlers map[EventType][]handlerEntry
	mu       sync.RWMutex
	nextID   int64
}

func NewDispatcher() Dispatcher {
	return &defaultDispatcher{
		handlers: make(map[EventType][]handlerEntry),
		nextID:   0,
	}
}

func (d *defaultDispatcher) Register(eventType EventType, handler EventHandler) HandlerID {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.nextID++
	id := HandlerID(fmt.Sprintf("handler_%d", d.nextID))

	if _, ok := d.handlers[eventType]; !ok {
		d.handlers[eventType] = []handlerEntry{}
	}

	entry := handlerEntry{
		id:      id,
		handler: handler,
	}

	d.handlers[eventType] = append(d.handlers[eventType], entry)
	return id
}

func (d *defaultDispatcher) Unregister(eventType EventType, handlerID HandlerID) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if handlers, ok := d.handlers[eventType]; ok {
		for i, entry := range handlers {
			if entry.id == handlerID {
				d.handlers[eventType] = append(handlers[:i], handlers[i+1:]...)
				break
			}
		}
	}
}

func (d *defaultDispatcher) Dispatch(event Event) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if handlers, ok := d.handlers[event.Type]; ok {
		for _, entry := range handlers {
			go entry.handler(event)
		}
	}

	if handlers, ok := d.handlers["*"]; ok {
		for _, entry := range handlers {
			go entry.handler(event)
		}
	}
}
