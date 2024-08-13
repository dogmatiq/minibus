package minibus

import (
	"context"
	"reflect"
)

// Subscribe configures the calling function to receive messages of type M in
// its inbox.
//
// It may only be called within a function that has been called by [Run]. It
// must be called before [Ready].
func Subscribe[M any](ctx context.Context) {
	f := caller(ctx)
	if f.ReadySignal == nil {
		panic("minibus: Subscribe() must not be called after calling Ready()")
	}

	f.Subscriptions.Add(f, reflect.TypeFor[M]())
}

// Ready signals that the function has made all relevant [Subscribe] calls and
// is ready to exchange messages.
//
// No messages are exchanged until all functions executed by the same call to
// [Run] have called [Ready].
func Ready(ctx context.Context) {
	f := caller(ctx)
	if f.ReadySignal == nil {
		return
	}

	select {
	case <-ctx.Done():
	case f.ReadySignal <- struct{}{}:
	}

	// We mark the function as ready even if the context is canceled, so that
	// any logic that checks the flag behaves the same regardless of we're going
	// to stop or keep running.
	f.ReadySignal = nil
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
