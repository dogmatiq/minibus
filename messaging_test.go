package minibus_test

import (
	"context"
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
				t.Log("component A")
				// Delay a little to help induce the race condition we're
				// testing for.
				time.Sleep(10 * time.Millisecond)

				Subscribe[string](ctx)

				started.Add(1)
				Start(ctx)
				t.Log("started component A")

				m, err := Receive(ctx)
				if err != nil {
					return err
				}

				if started.Load() != 3 {
					return fmt.Errorf("received a message before all components had started: %q", m)
				}

				t.Log("component A OK")

				return nil
			}),
			WithComponent(func(ctx context.Context) error {
				t.Log("component B")
				Subscribe[string](ctx)

				started.Add(1)
				Start(ctx)
				t.Log("started component B")

				m, err := Receive(ctx)
				if err != nil {
					return err
				}

				if started.Load() != 3 {
					return fmt.Errorf("received a message before all components had started: %q", m)
				}

				t.Log("component B OK")

				return nil
			}),
			WithComponent(func(ctx context.Context) error {
				t.Log("component C")
				started.Add(1)
				Start(ctx)
				t.Log("started component C")

				if err := Send(ctx, "<message>"); err != nil {
					return err
				}

				if started.Load() != 3 {
					return fmt.Errorf("sent a message before all components had started")
				}

				t.Log("component C OK")

				return nil
			}),
		)

		if err != nil {
			t.Fatalf("Run() returned an unexpected error: %s", err)
		}
	})
}
