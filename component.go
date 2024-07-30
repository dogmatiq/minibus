package minibus

import (
	"context"
	"reflect"
)

// Subscribe subscribes the calling component to receive messages of type M.
//
// It must be called before [Start].
func Subscribe[M any](ctx context.Context) {
	c := componentFromContext(ctx)
	if c.started == nil {
		panic("minibus: cannot subscribe after component has started")
	}

	t := reflect.TypeFor[M]()
	c.subscriptions = append(c.subscriptions, t)
}

// Start starts the calling component.
//
// No messages will be received until all components have started.
func Start(ctx context.Context) {
	c := componentFromContext(ctx)
	if c.started == nil {
		panic("minibus: component has already started")
	}

	c.started <- c
	c.started = nil
}

// Inbox returns a channel that receives messages sent to the calling component.
//
// No messages will be received until all components have started.
func Inbox(ctx context.Context) <-chan any {
	c := componentFromContext(ctx)
	if c.started != nil {
		panic("minibus: cannot receive messages before the component has started")
	}
	return c.inbox
}

// Outbox returns a channel that sends messages to other components.
func Outbox(ctx context.Context) chan<- any {
	c := componentFromContext(ctx)
	if c.started != nil {
		panic("minibus: cannot send messages before the component has started")
	}
	return c.outbox
}

// Send is a convenience function for sending a message to the bus, or aborting
// if ctx is canceled.
func Send[M any](ctx context.Context, m M) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case Outbox(ctx) <- m:
		return nil
	}
}

// Receive is a convenience function for receiving the next message from the
// bus, or aborting if ctx is canceled.
func Receive(ctx context.Context) (any, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case m := <-Inbox(ctx):
		return m, nil
	}
}

// A componenent is a participant in a messaging session.
type component struct {
	// subscriptions is a list of message types the component receives.
	subscriptions []reflect.Type

	// inbox is a channel that receives messages sent to the components.
	inbox chan any

	// output is a channel that sends messages to other components.
	outbox chan<- any

	// started is a channel used to indicate that the component completed its
	// subscriptions and is ready to send and receive messages.
	started chan<- *component

	// stopped is a channel that is closed when the component's run function
	// returns.
	stopped chan struct{}

	// err is the error returned by the component's run function, if any.
	err error
}

// contextKey is the key used to store a [component] within a [context.Context].
type contextKey struct{}

// contextWithComponent returns a new context with the given component attached.
func contextWithComponent(ctx context.Context, c *component) context.Context {
	return context.WithValue(ctx, contextKey{}, c)
}

// componentFromContext returns the component attached to the given context.
func componentFromContext(ctx context.Context) *component {
	if c, ok := ctx.Value(contextKey{}).(*component); ok {
		return c
	}
	panic("minibus: context was not created by minibus.Run()")
}
