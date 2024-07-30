package minibus

import (
	"context"
	"reflect"
)

// session is the context in which a set of functions are executed and exchange
// messages with each other.
type session struct {
	// inboxSize is the number of messages to buffer in each function's inbox.
	inboxSize int

	// funcs is the set of functions that are executed within the session.
	funcs map[*function]struct{}

	// ready is a channel on which functions signal when they are ready to
	// exchange messages.
	ready chan *function

	// returned is a channel on which functions signal when they have returned.
	returned chan *function

	// subscriptions is the set of all function's subscriptions, indexed by
	// message type.
	subscriptions map[reflect.Type]map[*function]struct{}

	// bus is the channel on which functions send bus to the session
	// for distribution to subscribers.
	bus chan envelope
}

// envelope is a container for a message and its meta-data.
type envelope struct {
	// sender is the function that sent the message.
	sender *function

	// message is the message itself.
	message any
}

// run executes all functions in parallel and exchanges messages between them.
func (s *session) run(ctx context.Context) error {
	if len(s.funcs) == 0 {
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
	for f := range s.funcs {
		go f.call(ctx)
	}

	pending := len(s.funcs)

	for pending > 0 {
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

		pending--
	}

	return nil
}

// stopFuncs signals all functions to return then blocks until they do.
func (s *session) stopFuncs() {
	for f := range s.funcs {
		close(f.inbox)
	}

	for f := range s.funcs {
		<-f.returned
	}
}

// addFuncSubscriptions merges the given function's subscriptions into the
// session's subscription index.
func (s *session) addFuncSubscriptions(f *function) {
	for _, t := range f.subscriptions {
		subs, ok := s.subscriptions[t]

		if !ok {
			subs = map[*function]struct{}{}

			if s.subscriptions == nil {
				s.subscriptions = map[reflect.Type]map[*function]struct{}{}
			}

			s.subscriptions[t] = subs
		}

		subs[f] = struct{}{}
	}
}

// removeFunc removes a function from the session, and removes its subscriptions
// from the session's subscription index.
func (s *session) removeFunc(f *function) {
	delete(s.funcs, f)
	for _, t := range f.subscriptions {
		delete(s.subscriptions[t], f)
	}
}

// exchangeMessages distributes messages between functions until all functions
// have returned, any single function returns an error, or ctx is canceled.
func (s *session) exchangeMessages(ctx context.Context) error {
	for f := range s.funcs {
		go f.pump(ctx)
	}

	for len(s.funcs) > 0 {
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
	subs := s.subscriptions[reflect.TypeOf(env.message)]

	for recipient := range subs {
		if recipient != env.sender {
			select {
			case recipient.inbox <- env.message:
			case <-recipient.returned:
			}
		}
	}
}
