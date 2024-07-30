package minibus_test

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/dogmatiq/minibus"
)

func TestRun_messaging(t *testing.T) {
	t.Run("it does not exchange any messages until all components have started", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		var started atomic.Int32

		err := Run(
			ctx,
			WithComponent(func(ctx context.Context) error {
				// Delay a little to help induce the race condition we're
				// testing for.
				time.Sleep(10 * time.Millisecond)

				Subscribe[string](ctx)

				started.Add(1)
				Start(ctx)

				m, err := Receive(ctx)
				if err != nil {
					return err
				}

				if started.Load() != 2 {
					return fmt.Errorf("received a message before all components had started: %q", m)
				}

				return nil
			}),
			WithComponent(func(ctx context.Context) error {
				started.Add(1)
				Start(ctx)

				if err := Send(ctx, "<message>"); err != nil {
					return err
				}

				if started.Load() != 2 {
					return fmt.Errorf("sent a message before all components had started")
				}

				return nil
			}),
		)

		if err != nil {
			t.Fatalf("Run() returned an unexpected error: %s", err)
		}
	})

	t.Run("it does not deliver messages to the component that sent them", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		err := Run(
			ctx,
			WithComponent(func(ctx context.Context) error {
				Subscribe[string](ctx)
				Start(ctx)

				err := Send(ctx, "<message>")
				if err != nil {
					return err
				}

				select {
				case <-time.After(50 * time.Millisecond):
					return nil
				case <-Inbox(ctx):
					return errors.New("component received a message from itself")
				}
			}),
		)

		if err != nil {
			t.Fatalf("Run() returned an unexpected error: %s", err)
		}
	})
}
