package events

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestNewDispatcher(t *testing.T) {
	dispatcher := NewDispatcher()
	if dispatcher == nil {
		t.Fatal("NewDispatcher should return a non-nil dispatcher")
	}
}

func TestDispatcher_Register(t *testing.T) {
	dispatcher := NewDispatcher()

	var received []Event
	var mu sync.Mutex

	handler := func(event Event) {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, event)
	}

	handlerID := dispatcher.Register(EventCDPCommand, handler)
	if handlerID == "" {
		t.Fatal("Register should return a non-empty HandlerID")
	}

	event := Event{
		Type:      EventCDPCommand,
		Method:    "Page.navigate",
		Params:    map[string]interface{}{"url": "https://example.com"},
		Timestamp: time.Now(),
	}

	dispatcher.Dispatch(event)

	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(received) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(received))
	}

	if received[0].Type != EventCDPCommand {
		t.Errorf("Expected event type %s, got %s", EventCDPCommand, received[0].Type)
	}
}

func TestDispatcher_RegisterMultipleHandlers(t *testing.T) {
	dispatcher := NewDispatcher()

	var received1, received2 []Event
	var mu1, mu2 sync.Mutex

	handler1 := func(event Event) {
		mu1.Lock()
		defer mu1.Unlock()
		received1 = append(received1, event)
	}

	handler2 := func(event Event) {
		mu2.Lock()
		defer mu2.Unlock()
		received2 = append(received2, event)
	}

	id1 := dispatcher.Register(EventCDPCommand, handler1)
	id2 := dispatcher.Register(EventCDPCommand, handler2)

	if id1 == id2 {
		t.Error("Different handlers should get different IDs")
	}

	event := Event{
		Type:      EventCDPCommand,
		Method:    "Page.navigate",
		Timestamp: time.Now(),
	}

	dispatcher.Dispatch(event)

	time.Sleep(10 * time.Millisecond)

	mu1.Lock()
	mu2.Lock()
	defer mu1.Unlock()
	defer mu2.Unlock()

	if len(received1) != 1 {
		t.Errorf("Handler1: Expected 1 event, got %d", len(received1))
	}

	if len(received2) != 1 {
		t.Errorf("Handler2: Expected 1 event, got %d", len(received2))
	}
}

func TestDispatcher_WildcardHandler(t *testing.T) {
	dispatcher := NewDispatcher()

	var received []Event
	var mu sync.Mutex

	wildcardHandler := func(event Event) {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, event)
	}

	dispatcher.Register("*", wildcardHandler)

	events := []Event{
		{Type: EventCDPCommand, Method: "Page.navigate", Timestamp: time.Now()},
		{Type: EventCDPEvent, Method: "Page.loadEventFired", Timestamp: time.Now()},
		{Type: EventClientConnected, SourceID: "client1", Timestamp: time.Now()},
	}

	for _, event := range events {
		dispatcher.Dispatch(event)
	}

	time.Sleep(20 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(received) != 3 {
		t.Fatalf("Expected 3 events, got %d", len(received))
	}

	receivedTypes := make(map[EventType]bool)
	for _, event := range received {
		receivedTypes[event.Type] = true
	}

	expectedTypes := []EventType{EventCDPCommand, EventCDPEvent, EventClientConnected}
	for _, expectedType := range expectedTypes {
		if !receivedTypes[expectedType] {
			t.Errorf("Expected to receive event type %s", expectedType)
		}
	}
}

func TestDispatcher_SpecificAndWildcardHandlers(t *testing.T) {
	dispatcher := NewDispatcher()

	var specificReceived, wildcardReceived []Event
	var mu1, mu2 sync.Mutex

	specificHandler := func(event Event) {
		mu1.Lock()
		defer mu1.Unlock()
		specificReceived = append(specificReceived, event)
	}

	wildcardHandler := func(event Event) {
		mu2.Lock()
		defer mu2.Unlock()
		wildcardReceived = append(wildcardReceived, event)
	}

	dispatcher.Register(EventCDPCommand, specificHandler)
	dispatcher.Register("*", wildcardHandler)

	event := Event{
		Type:      EventCDPCommand,
		Method:    "Page.navigate",
		Timestamp: time.Now(),
	}

	dispatcher.Dispatch(event)

	time.Sleep(10 * time.Millisecond)

	mu1.Lock()
	mu2.Lock()
	defer mu1.Unlock()
	defer mu2.Unlock()

	if len(specificReceived) != 1 {
		t.Errorf("Specific handler: Expected 1 event, got %d", len(specificReceived))
	}

	if len(wildcardReceived) != 1 {
		t.Errorf("Wildcard handler: Expected 1 event, got %d", len(wildcardReceived))
	}
}

func TestDispatcher_Unregister(t *testing.T) {
	dispatcher := NewDispatcher()

	var received []Event
	var mu sync.Mutex

	handler := func(event Event) {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, event)
	}

	handlerID := dispatcher.Register(EventCDPCommand, handler)
	dispatcher.Unregister(EventCDPCommand, handlerID)

	event := Event{
		Type:      EventCDPCommand,
		Method:    "Page.navigate",
		Timestamp: time.Now(),
	}

	dispatcher.Dispatch(event)

	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(received) != 0 {
		t.Errorf("Expected 0 events after unregister, got %d", len(received))
	}
}

func TestDispatcher_UnregisterNonExistentHandler(t *testing.T) {
	dispatcher := NewDispatcher()

	dispatcher.Unregister(EventCDPCommand, "nonexistent")
}

func TestDispatcher_UnregisterOneOfMultipleHandlers(t *testing.T) {
	dispatcher := NewDispatcher()

	var received1, received2 []Event
	var mu1, mu2 sync.Mutex

	handler1 := func(event Event) {
		mu1.Lock()
		defer mu1.Unlock()
		received1 = append(received1, event)
	}

	handler2 := func(event Event) {
		mu2.Lock()
		defer mu2.Unlock()
		received2 = append(received2, event)
	}

	id1 := dispatcher.Register(EventCDPCommand, handler1)
	dispatcher.Register(EventCDPCommand, handler2)

	dispatcher.Unregister(EventCDPCommand, id1)

	event := Event{
		Type:      EventCDPCommand,
		Method:    "Page.navigate",
		Timestamp: time.Now(),
	}

	dispatcher.Dispatch(event)

	time.Sleep(10 * time.Millisecond)

	mu1.Lock()
	mu2.Lock()
	defer mu1.Unlock()
	defer mu2.Unlock()

	if len(received1) != 0 {
		t.Errorf("Handler1: Expected 0 events after unregister, got %d", len(received1))
	}

	if len(received2) != 1 {
		t.Errorf("Handler2: Expected 1 event, got %d", len(received2))
	}
}

func TestDispatcher_DispatchNoHandlers(t *testing.T) {
	dispatcher := NewDispatcher()

	event := Event{
		Type:      EventCDPCommand,
		Method:    "Page.navigate",
		Timestamp: time.Now(),
	}

	dispatcher.Dispatch(event)
}

func TestDispatcher_ConcurrentAccess(t *testing.T) {
	dispatcher := NewDispatcher()

	var received []Event
	var mu sync.Mutex

	handler := func(event Event) {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, event)
	}

	var wg sync.WaitGroup
	var handlerIDs []HandlerID
	var idsMu sync.Mutex

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := dispatcher.Register(EventCDPCommand, handler)
			idsMu.Lock()
			handlerIDs = append(handlerIDs, id)
			idsMu.Unlock()
		}()
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			event := Event{
				Type:      EventCDPCommand,
				Method:    "Page.navigate",
				SourceID:  fmt.Sprintf("source_%d", index),
				Timestamp: time.Now(),
			}
			dispatcher.Dispatch(event)
		}(i)
	}

	wg.Wait()
	idsMu.Lock()
	for i := 0; i < len(handlerIDs)/2 && i < 5; i++ {
		wg.Add(1)
		go func(id HandlerID) {
			defer wg.Done()
			dispatcher.Unregister(EventCDPCommand, id)
		}(handlerIDs[i])
	}
	idsMu.Unlock()

	wg.Wait()

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(received) == 0 {
		t.Error("Expected to receive some events during concurrent access")
	}
}

func TestDispatcher_EventIntegrity(t *testing.T) {
	dispatcher := NewDispatcher()

	var received Event
	var mu sync.Mutex
	var eventReceived bool

	handler := func(event Event) {
		mu.Lock()
		defer mu.Unlock()
		received = event
		eventReceived = true
	}

	dispatcher.Register(EventWebhookTriggered, handler)

	originalEvent := Event{
		Type:       EventWebhookTriggered,
		Method:     "webhook.execute",
		Params:     map[string]interface{}{"id": "webhook123", "payload": "test"},
		SourceType: "webhook",
		SourceID:   "webhook123",
		Timestamp:  time.Now(),
	}

	dispatcher.Dispatch(originalEvent)

	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if !eventReceived {
		t.Fatal("Event was not received")
	}

	if received.Type != originalEvent.Type {
		t.Errorf("Type mismatch: expected %s, got %s", originalEvent.Type, received.Type)
	}

	if received.Method != originalEvent.Method {
		t.Errorf("Method mismatch: expected %s, got %s", originalEvent.Method, received.Method)
	}

	if received.SourceType != originalEvent.SourceType {
		t.Errorf("SourceType mismatch: expected %s, got %s", originalEvent.SourceType, received.SourceType)
	}

	if received.SourceID != originalEvent.SourceID {
		t.Errorf("SourceID mismatch: expected %s, got %s", originalEvent.SourceID, received.SourceID)
	}

	if len(received.Params) != len(originalEvent.Params) {
		t.Errorf("Params length mismatch: expected %d, got %d", len(originalEvent.Params), len(received.Params))
	}

	for key, value := range originalEvent.Params {
		if received.Params[key] != value {
			t.Errorf("Params mismatch for key %s: expected %v, got %v", key, value, received.Params[key])
		}
	}
}
