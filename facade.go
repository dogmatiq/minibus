package minibus

import (
	"context"
	"reflect"
)

// Run exchanges messages between functions that it executes in parallel.
//
// It blocks until all functions have returned, any single function returns an
// error, or ctx is canceled. Functions are added using the [WithFunc] option.
func Run(ctx context.Context, options ...Option) error {
	s := &session{
		inboxSize: 10,
		ready:     make(chan *function),
		returned:  make(chan *function),
		bus:       make(chan envelope),
	}

	for _, applyOption := range options {
		applyOption(s)
	}

	// Only create inboxes after all [WithInboxSize] options have been applied.
	for f := range s.funcs.Elements() {
		f.inbox = make(chan any, s.inboxSize)
	}

	return s.run(ctx)
}

// An Option is a function that configures the behavior of [Run].
type Option func(*session)

// WithFunc adds a function to be executed by a call to [Run].
func WithFunc(fn func(context.Context) error) Option {
	return func(s *session) {
		s.funcs.Add(&function{
			impl:     fn,
			session:  s,
			outbox:   make(chan any),
			returned: make(chan struct{}),
		})
	}
}

// WithInboxSize is an [Option] that sets number of messages that can be
// buffered in each function's inbox.
func WithInboxSize(size int) Option {
	return func(s *session) {
		s.inboxSize = size
	}
}

// Subscribe configures the calling function to receive messages of type M in
// its inbox.
//
// It may only be called within a function that has been called by [Run]. It
// must be called before [Ready].
func Subscribe[M any](ctx context.Context) {
	f := caller(ctx)
	if f.ready {
		panic("minibus: Subscribe() must not be called after calling Ready()")
	}

	t := reflect.TypeFor[M]()
	f.subscriptions.Add(t)
}

// Ready signals that the function has made all relevant [Subscribe] calls and
// is ready to exchange messages.
//
// No messages are exchanged until all functions executed by the same call to
// [Run] have called [Ready].
func Ready(ctx context.Context) {
	f := caller(ctx)
	if f.ready {
		return
	}

	select {
	case <-ctx.Done():
	case f.session.ready <- f:
	}

	f.ready = true
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
	return caller(ctx).inbox
}

// Outbox returns a channel on which the function can send messages to other
// functions executed by the same call to [Run].
//
// The channel will block until all functions executed by the same call to [Run]
// have called [Ready].
func Outbox(ctx context.Context) chan<- any {
	return caller(ctx).outbox
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
