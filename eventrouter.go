package stripeeventrouter

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/stripe/stripe-go/v84"
)

// handlerWrapper is an interface that wraps a handler function with unmarshaling
// It decouples the event routing logic from the specific event data types,
// allowing for type-safe handlers without exposing the generic implementation details.
type handlerWrapper interface {
	// Handle processes a Stripe event by unmarshaling its data and executing the handler
	Handle(event *stripe.Event) error
}

// CreateHandlerWrapper is a generic function that creates a typed handler wrapper
// It enables type-safe handling of Stripe event data by providing a generic handler
// for specific data types, eliminating the need for manual type assertions.
//
// Example usage:
//
//	// Register a handler for payment_intent.succeeded events
//	router.Register(
//	    stripe.EventTypePaymentIntentSucceeded,
//	    CreateHandlerWrapper(func(event *stripe.Event, paymentIntent *stripe.PaymentIntent) error {
//	        // Process the payment intent
//	        return nil
//	    }),
//	)
func CreateHandlerWrapper[T any](handler func(event *stripe.Event, data T) error) handlerWrapper {
	return &concreteHandlerWrapper[T]{
		handler: handler,
	}
}

// concreteHandlerWrapper implements handlerWrapper for a specific type T
// It handles the unmarshaling of event data into the appropriate type
// and delegates to the provided handler function.
type concreteHandlerWrapper[T any] struct {
	handler func(event *stripe.Event, data T) error
}

// Handle unmarshals the event data and calls the handler
// It takes care of converting the raw JSON data in the event to the expected type T,
// then passes both the original event and the typed data to the handler function.
func (w *concreteHandlerWrapper[T]) Handle(event *stripe.Event) error {
	if event == nil {
		return fmt.Errorf("event must not be nil")
	}
	if event.Data == nil {
		return fmt.Errorf("event.Data must not be nil")
	}
	var data T
	if err := json.Unmarshal(event.Data.Raw, &data); err != nil {
		return fmt.Errorf("failed to unmarshal event data: %w", err)
	}
	return w.handler(event, data)
}

// StripeEventRouter handles routing Stripe webhook events to registered handlers
// It provides a thread-safe registry of event handlers, allowing for easy
// registration and lookup of handlers for specific Stripe event types.
type Router struct {
	handlers map[stripe.EventType]handlerWrapper
	mu       sync.RWMutex // Protects concurrent access to the handlers map
}

// NewStripeEventRouter creates a new router for Stripe events
// It initializes an empty handler map ready for registering event handlers.
//
// Example usage:
//
//	router := NewStripeEventRouter()
func New() *Router {
	return &Router{
		handlers: make(map[stripe.EventType]handlerWrapper),
	}
}

// Register registers a handler for a specific Stripe event type
// This is a non-generic method that accepts a handlerWrapper, typically
// created using CreateHandlerWrapper.
//
// Example usage:
//
//	router.Register(
//	    stripe.EventTypeCustomerSubscriptionCreated,
//	    CreateHandlerWrapper(func(event *stripe.Event, sub *stripe.Subscription) error {
//	        // Process new subscription
//	        return nil
//	    }),
//	)
func (r *Router) Register(eventType stripe.EventType, wrapper handlerWrapper) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[eventType] = wrapper
}

// RouteEvent routes an event to the appropriate handler based on its type
// It looks up the registered handler for the event type and delegates processing.
// If no handler is registered for the event type, an error is returned.
//
// Example usage:
//
//	func HandleWebhook(w http.ResponseWriter, r *http.Request) {
//	    event, err := webhook.ConstructEvent(r.Body, stripeSignature, webhookSecret)
//	    if err != nil {
//	        // Handle invalid webhook
//	        return
//	    }
//
//	    if err := router.RouteEvent(&event); err != nil {
//	        // Handle routing error
//	        return
//	    }
//	}
func (r *Router) RouteEvent(event *stripe.Event) error {
	if event == nil {
		return fmt.Errorf("event must not be nil")
	}
	r.mu.RLock()
	handler, exists := r.handlers[event.Type]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no handler registered for event type: %s", event.Type)
	}

	return handler.Handle(event)
}

// RegisterMany registers the same handler for multiple Stripe event types at once
// This is a convenience function that avoids repetitive register calls when
// the same handler logic applies to multiple event types.
//
// Example usage:
//
//	// Register the same handler for multiple payment intent events
//	RegisterMany(
//	    router,
//	    []stripe.EventType{
//	        stripe.EventTypePaymentIntentCreated,
//	        stripe.EventTypePaymentIntentSucceeded,
//	        stripe.EventTypePaymentIntentCanceled,
//	    },
//	    func(event *stripe.Event, paymentIntent *stripe.PaymentIntent) error {
//	        // Handle any payment intent event
//	        log.WithContext(ctx).WithField("intent_id", paymentIntent.ID).
//	            Info("Processing payment intent event: " + string(event.Type))
//	        return nil
//	    },
//	)
func RegisterMany[T any](router *Router, eventTypes []stripe.EventType, handler func(event *stripe.Event, data T) error) {
	wrapper := CreateHandlerWrapper(handler)
	for _, eventType := range eventTypes {
		router.Register(eventType, wrapper)
	}
}
