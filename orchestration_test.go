package minibus_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/dogmatiq/minibus"
)

func TestRun(t *testing.T) {
	t.Run("it waits for all of the component functions", func(t *testing.T) {
		var calledA, calledB atomic.Bool

		if err := Run(
			context.Background(),
			WithComponent(func(context.Context) error {
				time.Sleep(10 * time.Millisecond)
				calledA.Store(true)
				return nil
			}),
			WithComponent(func(context.Context) error {
				time.Sleep(10 * time.Millisecond)
				calledB.Store(true)
				return nil
			}),
		); err != nil {
			t.Fatalf("Run() returned an unexpected error: %q", err)
		}

		if !calledA.Load() {
			t.Fatal("Run() did not call the first component function")
		}

		if !calledB.Load() {
			t.Fatal("Run() did not call the second component function")
		}
	})

	t.Run("it returns immediately when there are no components", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		if err := Run(ctx); err != nil {
			t.Fatalf("Run() returned an unexpected error: %q", err)
		}
	})

	t.Run("when a component returns an error", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		contextErr := make(chan error, 1)
		componentErr := errors.New("<error from component>")

		err := Run(
			ctx,
			WithComponent(func(ctx context.Context) error {
				<-ctx.Done()
				contextErr <- ctx.Err()
				return nil
			}),
			WithComponent(func(context.Context) error {
				return componentErr
			}),
		)

		t.Run("it returns the component's error", func(t *testing.T) {
			if err != componentErr {
				t.Fatalf("Run() returned an unexpected error: got %q, want %q", err, componentErr)
			}
		})

		t.Run("it cancels the context that it passes to the component functions", func(t *testing.T) {
			select {
			case err := <-contextErr:
				if err != context.Canceled {
					t.Fatalf("Run() did not cancel the context that it passed to the component functions: got %q, want %q", err, context.Canceled)
				}
			default:
				t.Fatalf("Run() did not cancel the context that it passed to the component functions")
			}
		})
	})

	t.Run("when the supplied context is canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		contextErr := make(chan error, 1)
		componentErr := errors.New("<error from component>")

		err := Run(
			ctx,
			WithComponent(func(ctx context.Context) error {
				<-ctx.Done()
				contextErr <- ctx.Err()
				return componentErr
			}),
		)

		t.Run("it returns the context error", func(t *testing.T) {
			if err != context.Canceled {
				t.Fatalf("Run() returned an unexpected error: got %q, want %q", err, context.Canceled)
			}
		})

		t.Run("it cancels the context that it passes to the component functions", func(t *testing.T) {
			select {
			case err := <-contextErr:
				if err != context.Canceled {
					t.Fatalf("Run() did not cancel the context that it passed to the component functions: got %q, want %q", err, context.Canceled)
				}
			default:
				t.Fatalf("Run() did not cancel the context that it passed to the component functions")
			}
		})
	})
}
