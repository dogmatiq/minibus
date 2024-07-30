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
	if c.started {
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
	if c.started {
		panic("minibus: component has already started")
	}

	select {
	case <-ctx.Done():
	case c.session.started <- c:
	}

	c.started = true
}

// Inbox returns a channel that receives messages sent to the calling component.
//
// No messages will be received until all components have started.
func Inbox(ctx context.Context) <-chan any {
	c := componentFromContext(ctx)
	if !c.started {
		panic("minibus: cannot receive messages before the component has started")
	}
	return c.inbox
}

// Outbox returns a channel that sends messages to other components.
func Outbox(ctx context.Context) chan<- any {
	c := componentFromContext(ctx)
	if !c.started {
		panic("minibus: cannot send messages before the component has started")
	}
	return c.outbox
}

// Send sends a message, or returns an error if ctx is canceled.
func Send[M any](ctx context.Context, m M) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case Outbox(ctx) <- m:
		return nil
	}
}

// Receive returns the next received message, or an error if ctx is canceled.
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
	// session is the messaging session the component is part of.
	session *session

	// runner is the function that implements the component's behavior.
	runner func(context.Context) error

	// subscriptions is a list of message types the component receives.
	subscriptions []reflect.Type

	// inbox is a channel that receives messages sent to the component.
	inbox chan any

	// outbox is a channel that sends messages to other components.
	outbox chan any

	// started is a bool that is set to true only after Start() is called.
	started bool

	// stopped is a channel that is closed when the component's run function
	// returns.
	stopped chan struct{}

	// err is the error returned by the component's run function, if any.
	err error
}

// contextKey is the key used to store a [component] within a [context.Context].
type contextKey struct{}

// componentFromContext returns the component attached to the given context.
func componentFromContext(ctx context.Context) *component {
	if c, ok := ctx.Value(contextKey{}).(*component); ok {
		return c
	}
	panic("minibus: context was not created by minibus.Run()")
}

func (c *component) run(ctx context.Context) {
	defer func() {
		close(c.stopped)

		select {
		case <-ctx.Done():
		case c.session.stopped <- c:
		}
	}()

	ctx = context.WithValue(ctx, contextKey{}, c)
	c.err = c.runner(ctx)
}

// pump pipes messages from the component's outbox to the session's message
// channel.
func (c *component) pump() {
	for {
		var m any

		select {
		case m = <-c.outbox:
		case <-c.stopped:
			return
		}

		select {
		case c.session.messages <- envelope{c, m}:
		case <-c.stopped:
			return
		}
	}
}
