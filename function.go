package minibus

import (
	"context"
	"reflect"
	"sync"
)

// A function represents an application-defined function that exchanges messages
// with other such functions.
type function struct {
	// Func is the application-defined function to execute.
	Func Func

	// Inbox and Outbox are the channels on which the function receives and
	// sends messages, respectively. Both channels block until all functions
	// have signalled readiness.
	Inbox, Outbox chan any

	// Subscriptions is the set of subscriptions for all "peers" of this
	// function. That is, the functions that may exchange messages with this
	// one.
	Subscriptions *subscriptions

	// Returned is a channel that is signaled when the function is ready to
	// exchange messages. It is set to nil when the function calls [Ready].
	ReadySignal chan<- struct{}

	// ReturnSignal is a channel that is signalled when the function has
	// returned.
	ReturnSignal chan<- functionResult

	// ReturnLatch is a channel that is closed when the function returns.
	ReturnLatch chan struct{}
}

type functionResult struct {
	Func *function
	Err  error
}

// callerKey is the key used to store a [function] within a [context.Context].
type callerKey struct{}

// caller returns the [function] attached to ctx.
func caller(ctx context.Context) *function {
	if f, ok := ctx.Value(callerKey{}).(*function); ok {
		return f
	}
	panic("minibus: context was not created by minibus.Run()")
}

// Call invokes the function and signals when it has returned.
func (f *function) Call(ctx context.Context) {
	ctx = context.WithValue(ctx, callerKey{}, f)

	err := f.Func(ctx)

	close(f.ReturnLatch)
	f.ReturnSignal <- functionResult{f, err}
}

func (f *function) Pump(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case m := <-f.Outbox:
			f.deliver(ctx, m)
		case <-f.ReturnLatch:
		}
	}
}

func (f *function) deliver(ctx context.Context, m any) {
	t := reflect.TypeOf(m)
	subs := f.Subscriptions.Subscribers(t)

	var g sync.WaitGroup

	for sub := range subs {
		if sub == f {
			continue
		}

		g.Add(1)

		go func() {
			defer g.Done()

			select {
			case <-ctx.Done():
			case <-sub.ReturnLatch:
			case sub.Inbox <- m:
			}
		}()
	}

	g.Wait()
}
