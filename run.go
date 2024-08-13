package minibus

import (
	"context"
	"sync"
)

// Func is a function that can be executed by [Run].
type Func func(context.Context) error

// Run exchanges messages between functions that it executes in parallel.
//
// It blocks until all functions have returned, any single function returns an
// error, or ctx is canceled. Functions are added using the [WithFunc] option.
func Run(
	ctx context.Context,
	functions ...Func,
) (err error) {
	running := map[*function]struct{}{}
	var pumps sync.WaitGroup

	subs := &subscriptions{}
	readySignal := make(chan struct{}, len(functions))
	returnSignal := make(chan functionResult, len(functions))

	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		// Cancel the context to signal functions AND message pumps to stop.
		cancel()

		// Wait for the message pumps to finish so we can guarantee that there
		// will be no more sends to any inboxes.
		pumps.Wait()

		// Close all of the inboxes to unblock functions that are readying from
		// their inbox without selecting on the context.
		for f := range running {
			close(f.Inbox)
		}

		// Wait for all remaining functions to return.
		for len(running) > 0 {
			r := <-returnSignal
			delete(running, r.Func)
		}
	}()

	// Call each function in it's own goroutine, and add it to a set of running
	// functions.
	for _, fn := range functions {
		f := &function{
			Func:          fn,
			Inbox:         make(chan any),
			Outbox:        make(chan any),
			Subscriptions: subs,
			ReadySignal:   readySignal,
			ReturnSignal:  returnSignal,
			ReturnLatch:   make(chan struct{}),
		}

		running[f] = struct{}{}

		go f.Call(ctx)
	}

	// Wait for all functions to signal readiness.
	readyCount := 0
	for readyCount < len(running) {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-readySignal:
			readyCount++

		case r := <-returnSignal:
			delete(running, r.Func)
			if r.Err != nil {
				return r.Err
			}
		}
	}

	// Start each functions message pump, unblocking the outbox channels, and
	// delivering to the inboxes.
	for f := range running {
		pumps.Add(1)
		go func() {
			defer pumps.Done()
			f.Pump(ctx)
		}()
	}

	// Wait for all running functions to return, or for an error to occur.
	for len(running) > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case r := <-returnSignal:
			delete(running, r.Func)
			if r.Err != nil {
				return r.Err
			}
		}
	}

	return ctx.Err()
}
