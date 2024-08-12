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

	log(ctx, "invoking %s", f)

	f.Err = f.impl(
		context.WithValue(ctx, callerKey{}, f),
	)
}

// Pump reads messages from the function's outbox, attaches meta-data then
// forwards the message to the session for distribution to subscribers.
//
// It implements an UNBOUNDED message queue such that publishing is never a
// blocking operation once the pump has started.
func (f *function) Pump(ctx context.Context) {
	if trace.IsEnabled() {
		var task *trace.Task
		ctx, task = trace.NewTask(ctx, "minibus.pump")
		defer task.End()
	}

	log(ctx, "starting outbox message pump for %s", f)

	var (
		queue  []envelope
		index  = 0
		outbox = f.Outbox
	)

	for {
		var (
			bus chan<- envelope
			env envelope
		)

		if len(queue) != 0 {
			bus = f.Session.Bus
			env = queue[index]
		} else if outbox == nil {
			return
		}

		select {
		case <-ctx.Done():
			return

		case m, ok := <-outbox:
			if !ok {
				outbox = nil
				continue
			}

			env := envelope{
				f,
				messageID.Add(1),
				messageType{reflect.TypeOf(m)},
				m,
			}

			queue = append(queue, env)
			log(ctx, "message %s#%d published by %s (queue: %d)", env.MessageType, env.MessageID, f, len(queue)-index)

		case bus <- env:
			queue[index] = envelope{}
			index++

			if index == len(queue) {
				queue = queue[:0]
				index = 0
			}

			log(ctx, "message %s#%d delivered to bus (queue: %d)", env.MessageType, env.MessageID, len(queue)-index)
		}
	}
}

func (f *function) Deliver(ctx context.Context, env envelope) {
	select {
	case <-ctx.Done():

	case f.Inbox <- env.Message:
		log(
			ctx,
			"message %s#%d delivered to %s",
			env.MessageType,
			env.MessageID,
			f,
		)

	case <-f.Returned:
		log(
			ctx,
			"message %s#%d not delivered to %s, function returned",
			env.MessageType,
			env.MessageID,
			f,
		)
	}
}

func (f *function) String() string {
	p := reflect.ValueOf(f.impl).Pointer()
	n := runtime.FuncForPC(p).Name()

	if i := strings.LastIndex(n, "/"); i != -1 {
		n = n[i+1:]
	}

	return strings.TrimSuffix(n, "-fm")
}
