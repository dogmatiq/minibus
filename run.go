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

	components := map[*component]struct{}{}
	started := make(chan *component, len(options.runners))
	stopped := make(chan *component, len(options.runners))

	realOutbox := make(chan any)
	subscriptions := map[reflect.Type]map[*component]struct{}{}

	ctx, cancel := context.WithCancel(ctx)

	defer func() {
		cancel()

		// Close the inbox of any remaining components and wait for them to
		// stop, ensuring none are left running, even if there is a panic.
		for c := range components {
			close(c.inbox)
			<-c.stopped
		}
	}()

	// Build components from the runners provided in the options.
	for _, run := range options.runners {
		c := &component{
			inbox:   make(chan any, options.buffer),
			outbox:  realOutbox,
			started: started,
			stopped: make(chan struct{}),
		}

		components[c] = struct{}{}

		// Start a goroutine for each component.
		go func() {
			defer func() {
				// Send the appropriate "component has stopped" signals.
				close(c.stopped) // per-component
				stopped <- c     // session-wide
			}()

			ctx := contextWithComponent(ctx, c)
			c.err = run(ctx)
		}()
	}

	// Keep track of the number of unstarted components so that we can avoid
	// processing the outbox until all components have started.
	unstarted := len(components)

	// Make the outbox appear to be nil until all components have started. This
	// ensures that no messages are published before components have setup their
	// subscriptions.
	var apparentOutbox <-chan any

	for len(components) > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case c := <-started:
			for _, t := range c.subscriptions {
				subs := subscriptions[t]
				if subs == nil {
					subs = map[*component]struct{}{}
					subscriptions[t] = subs
				}
				subs[c] = struct{}{}
			}

			unstarted--

			// Begin delivery of messages once all components have started.
			if unstarted == 0 {
				apparentOutbox = realOutbox
			}

		case c := <-stopped:
			close(c.inbox)
			delete(components, c)

			for _, t := range c.subscriptions {
				delete(subscriptions[t], c)
			}

			if c.err != nil {
				return c.err
			}

		case m := <-apparentOutbox:
			for c := range subscriptions[reflect.TypeOf(m)] {
				select {
				case c.inbox <- m:
				case <-c.stopped:
				}
			}
		}
	}

	return nil
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
