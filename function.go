package minibus

import (
	"context"
	"reflect"
	"runtime"
	"runtime/trace"
	"strings"
)

// A function represents an application-defined function that participates in a
// messaging session.
type function struct {
	// Session is the messaging Session that the function is executing within.
	Session *session

	// Subscriptions is the set of messages types that the function receives.
	Subscriptions map[messageType]struct{}

	// Ready is set to true when the function is Ready to exchange messages.
	Ready bool

	// Inbox is the channel on which the function receives its messages.
	Inbox chan any

	// Outbox is a channel on which the function may send messages.
	Outbox chan any

	// Returned is a channel that is closed when the function returns.
	Returned chan struct{}

	// Err is the error returned by the function, if any.
	Err error

	// impl is the actual function supplied by the application.
	impl func(context.Context) error
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
func (f *function) Call(ctx context.Context) {
	if trace.IsEnabled() {
		var task *trace.Task
		ctx, task = trace.NewTask(ctx, "minibus.func")
		defer task.End()
	}

	defer func() {
		close(f.Outbox)
		close(f.Returned)
		f.Session.Returned <- f
	}()

	trace.Logf(ctx, "minibus", "invoking %s", f)

	f.Err = f.impl(
		context.WithValue(ctx, callerKey{}, f),
	)
}

// Pump reads messages from the function's outbox, attaches meta-data then
// forwards the message to the session for distribution to subscribers.
func (f *function) Pump(ctx context.Context) {
	if trace.IsEnabled() {
		var task *trace.Task
		ctx, task = trace.NewTask(ctx, "minibus.pump")
		defer task.End()
	}

	trace.Logf(ctx, "minibus", "starting outbox message pump for %q", f)

	for m := range f.Outbox {
		env := envelope{
			f,
			messageType{reflect.TypeOf(m)},
			m,
		}

		trace.Logf(ctx, "minibus", "%q message published", env.MessageType)

		select {
		case <-ctx.Done():
			return
		case f.Session.Bus <- env:
		}
	}
}

func (f *function) String() string {
	p := reflect.ValueOf(f.impl).Pointer()
	n := runtime.FuncForPC(p).Name()

	if i := strings.LastIndex(n, "/"); i != -1 {
		n = n[i+1:]
	}

	return n
}
