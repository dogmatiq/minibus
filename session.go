package minibus

import (
	"context"
	"reflect"

	"github.com/dogmatiq/minibus/internal/set"
)

// session is the context in which a set of functions are executed and exchange
// messages with each other.
type session struct {
	// inboxSize is the number of messages to buffer in each function's inbox.
	inboxSize int

	// funcs is the set of functions that are executed within the session.
	funcs set.Set[*function]

	// ready is a channel on which functions signal when they are ready to
	// exchange messages.
	ready chan *function

	// returned is a channel on which functions signal when they have returned.
	returned chan *function

	// bus is the channel on which functions send bus to the session
	// for distribution to subscribers.
	bus chan envelope

	// subscribers is a map of message type to the set of functions that
	// subscribe to that message type.
	subscribers map[reflect.Type]*subscriberSet
}

// run executes all functions in parallel and exchanges messages between them.
func (s *session) run(ctx context.Context) error {
	if s.funcs.IsEmpty() {
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		s.stopFuncs()
	}()

	if err := s.startFuncs(ctx); err != nil {
		return err
	}

	return s.exchangeMessages(ctx)
}

// startFuncs calls all functions in their own goroutine and blocks until they are
// ready to exchange messages.
func (s *session) startFuncs(ctx context.Context) error {
	for f := range s.funcs.Elements() {
		go f.call(ctx)
	}

	for pending := s.funcs.LenX(); pending > 0; pending-- {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case f := <-s.ready:
			s.addFuncSubscriptions(f)

		case f := <-s.returned:
			s.removeFunc(f)
			if f.err != nil {
				return f.err
			}
		}
	}

	return nil
}

// stopFuncs signals all functions to return then blocks until they do.
func (s *session) stopFuncs() {
	for f := range s.funcs.Elements() {
		close(f.inbox)
	}

	for f := range s.funcs.Elements() {
		<-f.returned
	}
}

// addFuncSubscriptions merges the given function's subscriptions into the
// session's subscription indices.
func (s *session) addFuncSubscriptions(f *function) {
	for t := range f.subscriptions.Elements() {
		s.subscribersByType(t).Add(f)
	}
}

// removeFunc removes a function from the session, and removes its subscriptions
// from the session's subscription index.
func (s *session) removeFunc(f *function) {
	s.funcs.Remove(f)
	for t := range f.subscriptions.Elements() {
		s.subscribersByType(t).Remove(f)
	}
}

// exchangeMessages distributes messages between functions until all functions
// have returned, any single function returns an error, or ctx is canceled.
func (s *session) exchangeMessages(ctx context.Context) error {
	for f := range s.funcs.Elements() {
		go f.pump(ctx)
	}

	for !s.funcs.IsEmpty() {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case env := <-s.bus:
			s.deliverMessage(env)

		case f := <-s.returned:
			s.removeFunc(f)
			if f.err != nil {
				return f.err
			}
		}
	}

	return nil
}

// deliverMessage delivers the message in env to all relevant subscribers.
func (s *session) deliverMessage(env envelope) {
	publishedType := reflect.TypeOf(env.message)
	recipients := s.subscribersByType(publishedType)

	// Ensure we find any subscribers that are interested in the message type
	// due to a subscription to an interface that the message implements.
	if !recipients.isFinalized {
		for subscribedType, subscribers := range s.subscribers {
			if subscribedType.Kind() == reflect.Interface && publishedType.Implements(subscribedType) {
				recipients.AddSet(subscribers)
			}
		}

		recipients.isFinalized = true
	}

	// Deliver the message to all recipients.
	for recipient := range recipients.Elements() {
		if recipient != env.sender {
			select {
			case recipient.inbox <- env.message:
			case <-recipient.returned:
			}
		}
	}
}

func (s *session) subscribersByType(t reflect.Type) *subscriberSet {
	subs, ok := s.subscribers[t]

	if !ok {
		subs = &subscriberSet{
			isFinalized: t.Kind() == reflect.Interface,
		}

		if s.subscribers == nil {
			s.subscribers = map[reflect.Type]*subscriberSet{}
		}

		s.subscribers[t] = subs
	}

	return subs
}

// envelope is a container for a message and its meta-data.
type envelope struct {
	// sender is the function that sent the message.
	sender *function

	// message is the message itself.
	message any
}

// subscriberSet is a collection of the functions that subscribe to a particular
// message type.
type subscriberSet struct {
	set.Set[*function]

	// isFinalized is set to true once the subscribers set has been
	// updated to include functions that receive this message type because they
	// subscribe to an interface that it implements, as opposed to subscribing
	// to the concrete message type directly.
	isFinalized bool
}
