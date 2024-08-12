package minibus

import (
	"context"
	"reflect"
)

// Func is a function that can be executed by [Run].
type Func func(context.Context) error

// Run exchanges messages between functions that it executes in parallel.
//
// It blocks until all functions have returned, any single function returns an
// error, or ctx is canceled. Functions are added using the [WithFunc] option.
func Run(
	ctx context.Context,
	functions ...Func,
) error {
	s := &session{
		Ready:             make(chan *function),
		Returned:          make(chan *function),
		Bus:               make(chan envelope),
		funcs:             map[*function]struct{}{},
		subscribersByType: map[messageType]*subscribers{},
	}

	for _, fn := range functions {
		s.funcs[&function{
			Session:       s,
			Subscriptions: map[messageType]struct{}{},
			Inbox:         make(chan any),
			Outbox:        make(chan any),
			Returned:      make(chan struct{}),
			impl:          fn,
		}] = struct{}{}
	}

	return s.run(ctx)
}

// Subscribe configures the calling function to receive messages of type M in
// its inbox.
//
// It may only be called within a function that has been called by [Run]. It
// must be called before [Ready].
func Subscribe[M any](ctx context.Context) {
	f := caller(ctx)
	if f.Ready {
		panic("minibus: Subscribe() must not be called after calling Ready()")
	}

	t := messageType{reflect.TypeFor[M]()}
	f.Subscriptions[t] = struct{}{}
	log(ctx, "%s subscribed to %s", f, t)
}

// Ready signals that the function has made all relevant [Subscribe] calls and
// is ready to exchange messages.
//
// No messages are exchanged until all functions executed by the same call to
// [Run] have called [Ready].
func Ready(ctx context.Context) {
	f := caller(ctx)
	if f.Ready {
		return
	}

	select {
	case <-ctx.Done():
	case f.Session.Ready <- f:
	}

	f.Ready = true
}

// Inbox returns the channel on which the function receives messages send by
// other functions executed by the same call to [Run].
//
// Only messages with types matching those passed to [Subscribe] will be
// received.
//
// No messages are delivered until all functions executed by the same call to
// [Run] have called [Ready].
func Inbox(ctx context.Context) <-chan any {
	return caller(ctx).Inbox
}

// Outbox returns a channel on which the function can send messages to other
// functions executed by the same call to [Run].
//
// The channel will block until all functions executed by the same call to [Run]
// have called [Ready].
func Outbox(ctx context.Context) chan<- any {
	return caller(ctx).Outbox
}

// Send sends a message, or returns an error if ctx is canceled.
func Send(ctx context.Context, m any) error {
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
