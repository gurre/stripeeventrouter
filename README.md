# Stripe Webhook Events Router

[![Go Reference](https://pkg.go.dev/badge/github.com/gurre/stripeeventrouter.svg)](https://pkg.go.dev/github.com/gurre/stripeeventrouter)

## Problem

Processing Stripe webhooks in Go often leads to a single large `switch` on event type. As you add support for more events, that switch grows and becomes hard to read and maintain. This package replaces the switch with a router: you register one handler per event type (or per group of types), and the router dispatches each event to the right handler.

## Solution

Routes Stripe webhook events to registered handlers by event type. Handlers are type-safe: you receive `*stripe.PaymentIntent`, `*stripe.Charge`, and other Stripe types instead of raw JSON. The router is safe for concurrent use and works with events produced by Stripe’s `webhook.ConstructEvent`.

## Installation

```bash
go get github.com/gurre/stripeeventrouter
```

## Usage

Create a router, register handlers for the event types you care about, then pass each webhook event to the router.

### Basic flow

Create a router and register handlers; add your logic inside the handler (e.g. use `pi` for payment intent data). Call `RouteEvent` wherever you receive a Stripe event (e.g. in your webhook endpoint).

```go
router := stripeeventrouter.New()
router.Register(
	stripe.EventTypePaymentIntentSucceeded,
	stripeeventrouter.CreateHandlerWrapper(func(event *stripe.Event, pi *stripe.PaymentIntent) error {
		// your logic here; pi is already unmarshaled
		return nil
	}),
)
// where you receive the event (e.g. after webhook.ConstructEvent):
router.RouteEvent(&event)
```

### HTTP webhook handler

Use the same `router` you created and registered. Add error handling for `ReadAll`, `ConstructEvent`, and `RouteEvent` (e.g. return 400/500 and log).

```go
b, _ := io.ReadAll(r.Body)
event, _ := webhook.ConstructEvent(b, r.Header.Get("Stripe-Signature"), webhookSecret)
router.RouteEvent(&event)
w.WriteHeader(http.StatusOK)
```

### Same handler for multiple event types

Use `RegisterMany` to register one handler for several event types. Add your logic in the handler; use `event.Type` if you need to branch on which event fired.

```go
stripeeventrouter.RegisterMany(router,
	[]stripe.EventType{
		stripe.EventTypePaymentIntentCreated,
		stripe.EventTypePaymentIntentSucceeded,
		stripe.EventTypePaymentIntentCanceled,
	},
	func(event *stripe.Event, pi *stripe.PaymentIntent) error {
		// your logic here; same handler for all listed event types
		return nil
	},
)
```
