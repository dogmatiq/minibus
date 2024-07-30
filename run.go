package minibus

import (
	"context"
	"reflect"
)

// Run starts a messaging session.
//
// It blocks until all components (see [WithComponent]) have stopped, an error
// occurs, or ctx is canceled.
func Run(ctx context.Context, option ...RunOption) error {
	options := runOptions{buffer: 10}
	for _, opt := range option {
		opt(&options)
	}

	if len(options.runners) == 0 {
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	s := &session{
		components: map[*component]struct{}{},
		started:    make(chan *component),
		stopped:    make(chan *component),

		subscriptions: map[reflect.Type]map[*component]struct{}{},
		messages:      make(chan envelope),
	}

	for _, runner := range options.runners {
		c := &component{
			session: s,
			runner:  runner,
			inbox:   make(chan any, options.buffer),
			outbox:  make(chan any),
			stopped: make(chan struct{}),
		}
		s.components[c] = struct{}{}

		go c.run(ctx)
	}

	defer func() {
		cancel()

		// Close the inbox of any remaining components and wait for them to
		// stop, ensuring none are left running, even if there is a panic.
		for c := range s.components {
			close(c.inbox)
		}
		for c := range s.components {
			<-c.stopped
		}
	}()

	if err := s.waitForComponentsToStart(ctx); err != nil {
		return err
	}

	return s.deliverMessages(ctx)
}

func (s *session) waitForComponentsToStart(ctx context.Context) error {
	pending := len(s.components)

	for pending > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case c := <-s.started:
			s.handleStart(c)

		case c := <-s.stopped:
			if err := s.handleStop(c); err != nil {
				return err
			}
		}

		pending--
	}

	return nil
}

func (s *session) deliverMessages(ctx context.Context) error {
	for c := range s.components {
		go c.pump()
	}

	for len(s.components) > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case m := <-s.messages:
			s.handleDelivery(m)

		case c := <-s.stopped:
			if err := s.handleStop(c); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *session) handleStart(c *component) {
	for _, t := range c.subscriptions {
		subs := s.subscriptions[t]
		if subs == nil {
			subs = map[*component]struct{}{}
			s.subscriptions[t] = subs
		}
		subs[c] = struct{}{}
	}
}

func (s *session) handleStop(c *component) error {
	close(c.inbox)
	delete(s.components, c)

	for _, t := range c.subscriptions {
		delete(s.subscriptions[t], c)
	}

	return c.err
}

func (s *session) handleDelivery(env envelope) {
	subs := s.subscriptions[reflect.TypeOf(env.message)]

	for subscriber := range subs {
		if subscriber != env.publisher {
			select {
			case subscriber.inbox <- env.message:
			case <-subscriber.stopped:
			}
		}
	}
}

// RunOption is a functional option for configuring the behavior of [Run].
type RunOption func(*runOptions)

// WithComponent is a [RunOption] that adds an application component to the
// messaging session.
//
// run is called in its own goroutine. It may use the [Start], [Inbox],
// [Outbox], [Send] and [Receive] functions to perform messaging operations.
func WithComponent(run func(context.Context) error) RunOption {
	return func(opts *runOptions) {
		opts.runners = append(opts.runners, run)
	}
}

// WithBuffer is an [RunOption] that sets the buffer size for each component's
// inbox channel.
//
// The default buffer size is 10.
func WithBuffer(size int) RunOption {
	return func(opts *runOptions) {
		opts.buffer = size
	}
}

// runOptions is a collection of options for [Run], built by [RunOption]
// functions.
type runOptions struct {
	runners []func(context.Context) error
	buffer  int
}

// envelope is a container for a message and its meta-data.
type envelope struct {
	publisher *component
	message   any
}

type session struct {
	components map[*component]struct{}
	started    chan *component
	stopped    chan *component

	subscriptions map[reflect.Type]map[*component]struct{}
	messages      chan envelope
}
