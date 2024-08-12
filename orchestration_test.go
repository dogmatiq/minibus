package minibus_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/dogmatiq/minibus"
)

func TestRun_orchestration(t *testing.T) {
	t.Run("it waits for all of the functions to return", func(t *testing.T) {
		var calledA, calledB atomic.Bool

		err := Run(
			context.Background(),
			func(context.Context) error {
				time.Sleep(10 * time.Millisecond)
				calledA.Store(true)
				return nil
			},
			func(context.Context) error {
				time.Sleep(10 * time.Millisecond)
				calledB.Store(true)
				return nil
			},
		)

		if err != nil {
			t.Fatalf("Run() returned an unexpected error: %q", err)
		}

		if !calledA.Load() {
			t.Fatal("Run() did not call the first function")
		}

		if !calledB.Load() {
			t.Fatal("Run() did not call the second function")
		}
	})

	t.Run("it returns immediately when there are no functions to execute", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		if err := Run(ctx); err != nil {
			t.Fatalf("Run() returned an unexpected error: %q", err)
		}
	})

	t.Run("when a function returns an error", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		ctxErr := make(chan error, 1)
		funcErr := errors.New("<error from function>")

		err := Run(
			ctx,
			func(ctx context.Context) error {
				<-ctx.Done()
				ctxErr <- ctx.Err()
				return nil
			},
			func(context.Context) error {
				return funcErr
			},
		)

		t.Run("it returns the function's error", func(t *testing.T) {
			if err != funcErr {
				t.Fatalf("Run() returned an unexpected error: got %q, want %q", err, funcErr)
			}
		})

		t.Run("it cancels the context that it passes to the functions", func(t *testing.T) {
			select {
			case err := <-ctxErr:
				if err != context.Canceled {
					t.Fatalf("Run() did not cancel the context that it passed to the functions: got %q, want %q", err, context.Canceled)
				}
			default:
				t.Fatalf("Run() did not cancel the context that it passed to the functions")
			}
		})
	})

	t.Run("when the supplied context is canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		ctxErr := make(chan error, 1)
		funcErr := errors.New("<error from function>")

		err := Run(
			ctx,
			func(ctx context.Context) error {
				<-ctx.Done()
				ctxErr <- ctx.Err()
				return funcErr
			},
		)

		t.Run("it returns the context error", func(t *testing.T) {
			if err != context.Canceled {
				t.Fatalf("Run() returned an unexpected error: got %q, want %q", err, context.Canceled)
			}
		})

		t.Run("it cancels the context that it passes to the functions", func(t *testing.T) {
			select {
			case err := <-ctxErr:
				if err != context.Canceled {
					t.Fatalf("Run() did not cancel the context that it passed to the functions: got %q, want %q", err, context.Canceled)
				}
			default:
				t.Fatalf("Run() did not cancel the context that it passed to the functions")
			}
		})
	})
}
