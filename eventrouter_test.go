package stripeeventrouter

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/stripe/stripe-go/v82"
)

// mockEvent creates a mock Stripe event with the given type and data
func mockEvent(t *testing.T, eventType stripe.EventType, data interface{}) *stripe.Event {
	rawData, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	return &stripe.Event{
		Type: eventType,
		Data: &stripe.EventData{
			Raw: rawData,
		},
	}
}

func TestRegisterAndRouteEvent(t *testing.T) {
	router := New()

	// Mock data for testing
	mockCharge := &stripe.Charge{
		ID:     "ch_123456",
		Amount: 1000,
	}

	// Keep track of which handlers were called
	var chargeHandlerCalled bool
	var receivedCharge *stripe.Charge

	// Create handler for charge events
	chargeHandler := CreateHandlerWrapper(func(event *stripe.Event, charge stripe.Charge) error {
		chargeHandlerCalled = true
		receivedCharge = &charge
		return nil
	})

	// Register the handler
	router.Register(stripe.EventTypeChargeSucceeded, chargeHandler)

	// Create a mock event
	event := mockEvent(t, stripe.EventTypeChargeSucceeded, mockCharge)

	// Route the event
	err := router.RouteEvent(event)
	if err != nil {
		t.Fatalf("RouteEvent returned error: %v", err)
	}

	// Check that the correct handler was called
	if !chargeHandlerCalled {
		t.Error("Charge handler was not called")
	}

	// Check that the data was correctly unmarshaled
	if receivedCharge == nil {
		t.Fatal("Received charge is nil")
	}
	if receivedCharge.ID != mockCharge.ID {
		t.Errorf("Expected charge ID %s, got %s", mockCharge.ID, receivedCharge.ID)
	}
	if receivedCharge.Amount != mockCharge.Amount {
		t.Errorf("Expected charge amount %d, got %d", mockCharge.Amount, receivedCharge.Amount)
	}
}

func TestUnregisteredEventType(t *testing.T) {
	router := New()

	// Create a mock event with an unregistered type
	event := mockEvent(t, stripe.EventTypeInvoiceCreated, struct{}{})

	// Route the event - should fail
	err := router.RouteEvent(event)
	if err == nil {
		t.Error("Expected error for unregistered event type, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "no handler registered") {
		t.Errorf("Expected error to contain 'no handler registered', got: %v", err)
	}
	if err != nil && !strings.Contains(err.Error(), string(stripe.EventTypeInvoiceCreated)) {
		t.Errorf("Expected error to contain event type %s, got: %v", stripe.EventTypeInvoiceCreated, err)
	}
}

func TestHandlerError(t *testing.T) {
	router := New()

	// Create a handler that returns an error
	expectedError := errors.New("handler error")
	errorHandler := CreateHandlerWrapper(func(event *stripe.Event, data stripe.Charge) error {
		return expectedError
	})

	// Register the handler
	router.Register(stripe.EventTypeChargeSucceeded, errorHandler)

	// Route an event to the handler
	event := mockEvent(t, stripe.EventTypeChargeSucceeded, &stripe.Charge{})
	err := router.RouteEvent(event)

	// Verify that the error was propagated
	if err != expectedError {
		t.Errorf("Expected error %v, got %v", expectedError, err)
	}
}

func TestUnmarshalError(t *testing.T) {
	router := New()

	// Register a handler expecting a Charge
	handler := CreateHandlerWrapper(func(event *stripe.Event, charge stripe.Charge) error {
		return nil
	})
	router.Register(stripe.EventTypeChargeSucceeded, handler)

	// Create an event with invalid JSON
	event := &stripe.Event{
		Type: stripe.EventTypeChargeSucceeded,
		Data: &stripe.EventData{
			Raw: []byte(`{invalid json}`),
		},
	}

	// Route the event - should fail with unmarshal error
	err := router.RouteEvent(event)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("Expected error to contain 'unmarshal', got: %v", err)
	}
}

func TestRouteEventNilEvent(t *testing.T) {
	router := New()
	err := router.RouteEvent(nil)
	if err == nil {
		t.Error("Expected error for nil event, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "must not be nil") {
		t.Errorf("Expected error to contain 'must not be nil', got: %v", err)
	}
}

func TestHandleNilEvent(t *testing.T) {
	handler := CreateHandlerWrapper(func(event *stripe.Event, data stripe.Charge) error {
		return nil
	})
	err := handler.Handle(nil)
	if err == nil {
		t.Error("Expected error for nil event, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "must not be nil") {
		t.Errorf("Expected error to contain 'must not be nil', got: %v", err)
	}
}

func TestHandleNilEventData(t *testing.T) {
	handler := CreateHandlerWrapper(func(event *stripe.Event, data stripe.Charge) error {
		return nil
	})
	event := &stripe.Event{
		Type: stripe.EventTypeChargeSucceeded,
		Data: nil,
	}
	err := handler.Handle(event)
	if err == nil {
		t.Error("Expected error for nil event.Data, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "event.Data") {
		t.Errorf("Expected error to mention event.Data, got: %v", err)
	}
}

func TestRegisterMultipleHandlers(t *testing.T) {
	router := New()

	// Track which handlers were called
	var chargeHandlerCalled, payoutHandlerCalled bool

	// Register handlers for different event types
	chargeHandler := CreateHandlerWrapper(func(event *stripe.Event, charge stripe.Charge) error {
		chargeHandlerCalled = true
		return nil
	})
	payoutHandler := CreateHandlerWrapper(func(event *stripe.Event, payout stripe.Payout) error {
		payoutHandlerCalled = true
		return nil
	})

	router.Register(stripe.EventTypeChargeSucceeded, chargeHandler)
	router.Register(stripe.EventTypePayoutCreated, payoutHandler)

	// Route a charge event
	chargeEvent := mockEvent(t, stripe.EventTypeChargeSucceeded, &stripe.Charge{})
	err := router.RouteEvent(chargeEvent)
	if err != nil {
		t.Fatalf("RouteEvent for charge returned error: %v", err)
	}

	// Route a payout event
	payoutEvent := mockEvent(t, stripe.EventTypePayoutCreated, &stripe.Payout{})
	err = router.RouteEvent(payoutEvent)
	if err != nil {
		t.Fatalf("RouteEvent for payout returned error: %v", err)
	}

	// Check that the correct handlers were called
	if !chargeHandlerCalled {
		t.Error("Charge handler was not called")
	}
	if !payoutHandlerCalled {
		t.Error("Payout handler was not called")
	}
}

func TestOverwriteHandler(t *testing.T) {
	router := New()

	// Create two handlers for the same event type
	var handler1Called, handler2Called bool
	handler1 := CreateHandlerWrapper(func(event *stripe.Event, charge stripe.Charge) error {
		handler1Called = true
		return nil
	})
	handler2 := CreateHandlerWrapper(func(event *stripe.Event, charge stripe.Charge) error {
		handler2Called = true
		return nil
	})

	// Register the first handler
	router.Register(stripe.EventTypeChargeSucceeded, handler1)

	// Register a second handler for the same event type (should overwrite)
	router.Register(stripe.EventTypeChargeSucceeded, handler2)

	// Route a charge event
	event := mockEvent(t, stripe.EventTypeChargeSucceeded, &stripe.Charge{})
	err := router.RouteEvent(event)
	if err != nil {
		t.Fatalf("RouteEvent returned error: %v", err)
	}

	// Only the second handler should be called
	if handler1Called {
		t.Error("First handler was called after being overwritten")
	}
	if !handler2Called {
		t.Error("Second handler was not called")
	}
}

func TestConcurrentAccess(t *testing.T) {
	router := New()

	// Register a handler
	var counter int
	var mu sync.Mutex
	handler := CreateHandlerWrapper(func(event *stripe.Event, charge stripe.Charge) error {
		mu.Lock()
		counter++
		mu.Unlock()
		return nil
	})
	router.Register(stripe.EventTypeChargeSucceeded, handler)

	// Create a mock event
	event := mockEvent(t, stripe.EventTypeChargeSucceeded, &stripe.Charge{})

	// Run multiple goroutines registering handlers and routing events
	var wg sync.WaitGroup
	const numGoroutines = 10
	const numOperations = 100

	// Add goroutines that route events
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				err := router.RouteEvent(event)
				if err != nil {
					t.Errorf("RouteEvent returned error: %v", err)
				}
			}
		}()
	}

	// Add goroutines that register handlers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				// Each goroutine registers a different event type
				eventType := stripe.EventType(string(stripe.EventTypeChargeSucceeded) + string(rune(i)))
				router.Register(eventType, handler)
			}
		}(i)
	}

	wg.Wait()

	// The counter should equal numGoroutines * numOperations
	mu.Lock()
	defer mu.Unlock()
	expectedCount := numGoroutines * numOperations
	if counter != expectedCount {
		t.Errorf("Expected counter to be %d, got %d", expectedCount, counter)
	}
}

func TestCreateHandlerWrapper(t *testing.T) {
	// Test that createHandlerWrapper correctly preserves the type
	type TestData struct {
		Field1 string
		Field2 int
	}

	expectedData := TestData{
		Field1: "test",
		Field2: 123,
	}

	var receivedData *TestData
	handler := CreateHandlerWrapper(func(event *stripe.Event, data TestData) error {
		receivedData = &data
		return nil
	})

	rawData, _ := json.Marshal(expectedData)
	event := &stripe.Event{
		Data: &stripe.EventData{
			Raw: rawData,
		},
	}

	err := handler.Handle(event)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if receivedData == nil {
		t.Fatal("Received data is nil")
	}
	if receivedData.Field1 != expectedData.Field1 {
		t.Errorf("Expected Field1 to be %s, got %s", expectedData.Field1, receivedData.Field1)
	}
	if receivedData.Field2 != expectedData.Field2 {
		t.Errorf("Expected Field2 to be %d, got %d", expectedData.Field2, receivedData.Field2)
	}
}

func TestRegisterMany(t *testing.T) {
	// Create a router
	router := New()

	// Define test event types
	eventTypes := []stripe.EventType{
		stripe.EventTypePaymentIntentSucceeded,
		stripe.EventTypePaymentIntentCanceled,
		stripe.EventTypePaymentIntentCreated,
	}

	// Track which event types have been handled
	handledEvents := make(map[stripe.EventType]bool)

	// Register a handler for multiple event types
	RegisterMany(
		router,
		eventTypes,
		func(event *stripe.Event, paymentIntent *stripe.PaymentIntent) error {
			handledEvents[event.Type] = true
			// Verify the data was properly unmarshaled
			if paymentIntent.ID != "pi_test_123" {
				t.Errorf("Expected payment intent ID to be pi_test_123, got %s", paymentIntent.ID)
			}
			return nil
		},
	)

	// Create mock payment intent data
	mockPI := &stripe.PaymentIntent{
		ID: "pi_test_123",
	}
	rawData, err := json.Marshal(mockPI)
	if err != nil {
		t.Fatalf("Failed to marshal mock data: %v", err)
	}

	// Test that all event types are handled
	for _, eventType := range eventTypes {
		// Create a mock event
		event := &stripe.Event{
			Type: eventType,
			Data: &stripe.EventData{
				Raw: rawData,
			},
		}

		// Route the event
		err := router.RouteEvent(event)
		if err != nil {
			t.Errorf("Failed to route event %s: %v", eventType, err)
		}

		// Verify the event was handled
		if !handledEvents[eventType] {
			t.Errorf("Event %s was not handled", eventType)
		}
	}

	// Test that an unregistered event type returns an error
	unregisteredEvent := &stripe.Event{
		Type: stripe.EventTypeCustomerSubscriptionCreated,
	}

	err = router.RouteEvent(unregisteredEvent)
	if err == nil {
		t.Error("Expected error for unregistered event type, got nil")
	}
}

func TestRegisterManyEmptySlice(t *testing.T) {
	router := New()
	RegisterMany(router, []stripe.EventType{}, func(event *stripe.Event, pi *stripe.PaymentIntent) error {
		return nil
	})
	event := mockEvent(t, stripe.EventTypePaymentIntentSucceeded, &stripe.PaymentIntent{ID: "pi_123"})
	err := router.RouteEvent(event)
	if err == nil {
		t.Error("Expected error when no handlers registered via empty slice, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "no handler registered") {
		t.Errorf("Expected error to contain 'no handler registered', got: %v", err)
	}
}

func TestRegisterManyWithMultipleHandlers(t *testing.T) {
	// Create a router
	router := New()

	// Track which handlers were called
	var paymentHandlerCalled, chargeHandlerCalled bool

	// Register handlers for payment events
	paymentEventTypes := []stripe.EventType{
		stripe.EventTypePaymentIntentSucceeded,
		stripe.EventTypePaymentIntentCanceled,
	}

	RegisterMany(
		router,
		paymentEventTypes,
		func(event *stripe.Event, paymentIntent *stripe.PaymentIntent) error {
			paymentHandlerCalled = true
			return nil
		},
	)

	// Register handlers for charge events
	chargeEventTypes := []stripe.EventType{
		stripe.EventTypeChargeSucceeded,
		stripe.EventTypeChargeFailed,
	}

	RegisterMany(
		router,
		chargeEventTypes,
		func(event *stripe.Event, charge *stripe.Charge) error {
			chargeHandlerCalled = true
			return nil
		},
	)

	// Create mock data
	mockPaymentIntent, _ := json.Marshal(&stripe.PaymentIntent{ID: "pi_test"})
	mockCharge, _ := json.Marshal(&stripe.Charge{ID: "ch_test"})

	// Test payment event routing
	paymentEvent := &stripe.Event{
		Type: stripe.EventTypePaymentIntentSucceeded,
		Data: &stripe.EventData{
			Raw: mockPaymentIntent,
		},
	}

	err := router.RouteEvent(paymentEvent)
	if err != nil {
		t.Errorf("Failed to route payment event: %v", err)
	}

	if !paymentHandlerCalled {
		t.Error("Payment handler was not called")
	}

	// Test charge event routing
	chargeEvent := &stripe.Event{
		Type: stripe.EventTypeChargeSucceeded,
		Data: &stripe.EventData{
			Raw: mockCharge,
		},
	}

	err = router.RouteEvent(chargeEvent)
	if err != nil {
		t.Errorf("Failed to route charge event: %v", err)
	}

	if !chargeHandlerCalled {
		t.Error("Charge handler was not called")
	}
}
