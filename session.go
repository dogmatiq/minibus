package minibus

import (
	"context"
	"reflect"
	"runtime/trace"
	"sync"
)

// session is the context in which a set of functions are executed and exchange
// messages with each other.
type session struct {
	// InboxSize is the number of messages to buffer in each function's inbox.
	InboxSize int

	// Ready is a channel on which functions signal when they are Ready to
	// exchange messages.
	Ready chan *function

	// Returned is a channel on which functions signal when they have Returned.
	Returned chan *function

	// Bus is the channel on which functions send Bus to the session
	// for distribution to subscribers.
	Bus chan envelope

	// funcs is the set of functions that are executed within the session.
	funcs map[*function]struct{}

	// subscribersByType is a map of message type to the set of functions that
	// subscribe to that message type.
	subscribersByType map[messageType]*subscribers
}

// run executes all functions in parallel and exchanges messages between them.
func (s *session) run(ctx context.Context) error {
	if trace.IsEnabled() {
		var task *trace.Task
		ctx, task = trace.NewTask(ctx, "minibus.session")
		defer task.End()
	}

	trace.Logf(ctx, "minibus", "session started with %d function(s)", len(s.funcs))

	if len(s.funcs) == 0 {
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		s.stopFuncs(ctx)
	}()

	if err := s.startFuncs(ctx); err != nil {
		return err
	}

	return s.exchangeMessages(ctx)
}

// startFuncs calls all functions in their own goroutine and blocks until they are
// ready to exchange messages.
func (s *session) startFuncs(ctx context.Context) error {
	for f := range s.funcs {
		go f.Call(ctx)
	}

	for pending := len(s.funcs); pending > 0; pending-- {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case f := <-s.Ready:
			s.onReady(ctx, f)

		case f := <-s.Returned:
			s.onReturn(ctx, f)
			if f.Err != nil {
				return f.Err
			}
		}
	}

	return nil
}

// stopFuncs signals all functions to return then blocks until they do.
func (s *session) stopFuncs(ctx context.Context) {
	for f := range s.funcs {
		close(f.Inbox)
		trace.Logf(ctx, "minibus", "closed inbox for %s", f)
	}

	for len(s.funcs) > 0 {
		f := <-s.Returned
		s.onReturn(ctx, f)
	}
}

// onReady merges the given function's subscriptions into the
// session's subscription indices.
func (s *session) onReady(ctx context.Context, f *function) {
	for t := range f.Subscriptions {
		s.subscribers(t).Members[f] = struct{}{}
	}
	if len(f.Subscriptions) > 0 {
		trace.Logf(ctx, "minibus", "merged %d subscription(s) into subscription index from %s", len(f.Subscriptions), f)
	}
}

// onReturn removes a function from the session, and removes its subscriptions
// from the session's subscription index.
func (s *session) onReturn(ctx context.Context, f *function) {
	for t := range f.Subscriptions {
		delete(s.subscribers(t).Members, f)
	}
	delete(s.funcs, f)

	if f.Err == nil {
		trace.Logf(ctx, "minibus", "%s returned successfully", f)
	} else {
		trace.Logf(ctx, "minibus", "%s returned %T error: %q", f, f.Err, f.Err)
	}
}

// exchangeMessages distributes messages between functions until all functions
// have returned, any single function returns an error, or ctx is canceled.
func (s *session) exchangeMessages(ctx context.Context) error {
	for f := range s.funcs {
		go f.Pump(ctx)
	}

	trace.Logf(ctx, "minibus", "message exchange started with %d function(s)", len(s.funcs))

	for len(s.funcs) != 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case env := <-s.Bus:
			if err := s.deliverMessage(ctx, env); err != nil {
				return err
			}

		case f := <-s.Returned:
			s.onReturn(ctx, f)
			if f.Err != nil {
				return f.Err
			}
		}
	}

	return nil
}

// deliverMessage delivers the message in env to all relevant subscribers.
func (s *session) deliverMessage(ctx context.Context, env envelope) error {
	recipients := s.subscribers(env.MessageType)

	if !recipients.IsFinalized {
		for subscribedType, subscribers := range s.subscribersByType {
			if subscribedType.Kind() == reflect.Interface && env.MessageType.Implements(subscribedType) {
				for f := range subscribers.Members {
					recipients.Members[f] = struct{}{}
					f.Subscriptions[env.MessageType] = struct{}{}
				}
				trace.Logf(ctx, "minibus", "updated subscription index for %q to include %q, %d functions(s) subscribed", subscribedType, env.MessageType, len(subscribers.Members))
			}
		}

		recipients.IsFinalized = true
	}

	var g sync.WaitGroup

	for recipient := range recipients.Members {
		if recipient != env.Publisher {
			g.Add(1)

			go func() {
				defer g.Done()

				select {
				case <-ctx.Done():
				case recipient.Inbox <- env.Message:
					trace.Logf(ctx, "minibus", "delivered %q message to %s", env.MessageType, recipient)
				case <-recipient.Returned:
					trace.Logf(ctx, "minibus", "%s returned before %q message could be delivered", recipient, env.MessageType)
				}
			}()
		}
	}

	g.Wait()

	return ctx.Err()
}

// subscribers returns the subscriber set for the given message type, creating
// it if necessary.
func (s *session) subscribers(t messageType) *subscribers {
	subs, ok := s.subscribersByType[t]

	if !ok {
		subs = &subscribers{
			Members:     map[*function]struct{}{},
			IsFinalized: t.Kind() == reflect.Interface,
		}
		s.subscribersByType[t] = subs
	}

	return subs
}

// envelope is a container for a message and its meta-data.
type envelope struct {
	// Publisher is the function that sent the message.
	Publisher *function

	// MessageType is the type of the message.
	MessageType messageType

	// Message is the message itself.
	Message any
}

// subscribers is a collection of the functions that subscribe to a particular
// message type.
type subscribers struct {
	Members map[*function]struct{}

	// IsFinalized is set to true once the subscribers set has been
	// updated to include functions that receive this message type because they
	// subscribe to an interface that it implements, as opposed to subscribing
	// to the concrete message type directly.
	IsFinalized bool
}

type messageType struct{ reflect.Type }

func (t messageType) String() string {
	n := t.Type.String()
	if n == "interface {}" {
		return "any"
	}
	return n
}
