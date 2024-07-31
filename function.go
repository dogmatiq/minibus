package minibus

import (
	"context"
	"reflect"

	"github.com/dogmatiq/minibus/internal/set"
)

// A function represents an application-defined function that participates in a
// messaging session.
type function struct {
	// impl is the actual function supplied by the application.
	impl func(context.Context) error

	// session is the messaging session that the function is executing within.
	session *session

	// subscriptions is the set of messages types that the function receives.
	subscriptions set.Set[reflect.Type]

	// ready is set to true when the function is ready to exchange messages.
	ready bool

	// inbox is the channel on which the function receives its messages.
	inbox chan any

	// outbox is a channel on which the function may send messages.
	outbox chan any

	// returned is a channel that is closed when the function returns.
	returned chan struct{}

	// err is the error returned by the function, if any.
	err error
}

// callerKey is the key used to store a [function] within a [context.Context].
type callerKey struct{}

// caller returns the [function] attached to ctx.
func caller(ctx context.Context) *function {
	if f, ok := ctx.Value(callerKey{}).(*function); ok {
		return f
	}
	panic("minibus: context was not created by Run()")
}

// call invokes the function and signals when it has returned.
func (f *function) call(ctx context.Context) {
	defer func() {
		close(f.outbox)
		close(f.returned)

		select {
		case <-ctx.Done():
		case f.session.returned <- f:
		}
	}()

	ctx = context.WithValue(ctx, callerKey{}, f)
	f.err = f.impl(ctx)
}

// pump reads messages from the function's outbox, attaches meta-data then
// forwards the message to the session for distribution to subscribers.
func (f *function) pump(ctx context.Context) {
	for m := range f.outbox {
		select {
		case <-ctx.Done():
			return
		case f.session.bus <- envelope{f, m}:
		}
	}
}
