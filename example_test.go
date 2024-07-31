package minibus_test

import (
	"context"
	"fmt"
	"time"

	"github.com/dogmatiq/minibus"
)

func Example() {
	// SayHello is an example message type. Minibus doesn't care what types you
	// use for messages, but it's typical to use struct types.
	type SayHello struct {
		Name string
	}

	// The recipient function handles SayHello messages.
	recipient := func(ctx context.Context) error {
		// Subscribe to the types of messages we're interested in.
		minibus.Subscribe[SayHello](ctx)

		// All functions must signal readiness before messages are exchanged.
		minibus.Ready(ctx)

		// Handle the messages received on the function's inbox channel. It will
		// only receive messages of the same type that it subscribed to.
		//
		// The inbox channel is closed when ctx.Done() is closed, so it's not
		// necessary to select on both.
		for m := range minibus.Inbox(ctx) {
			switch m := m.(type) {
			case SayHello:
				fmt.Printf("Hello, %s!\n", m.Name)

				// We've said our greetings, let's get out of here.
				return nil
			}
		}

		// If the inbox channel was closed before we received the message it
		// means we were signalled to stop.
		return nil
	}

	// The sender function sends a SayHello message to the other functions.
	sender := func(ctx context.Context) error {
		minibus.Ready(ctx)
		return minibus.Send(ctx, SayHello{"world"})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// The Run function executes each function in its own goroutine and
	// exchanges messages between them. It blocks until all functions return.
	if err := minibus.Run(
		ctx,
		minibus.WithFunc(recipient),
		minibus.WithFunc(sender),
	); err != nil {
		fmt.Println(err)
	}

	// Output:
	// Hello, world!
}

func ExampleSubscribe_fireHose() {
	recipient := func(ctx context.Context) error {
		// Receive everything by subscribing to the [any] interface.
		minibus.Subscribe[any](ctx)
		minibus.Ready(ctx)

		for m := range minibus.Inbox(ctx) {
			fmt.Println(m)
		}

		return nil
	}

	sender := func(ctx context.Context) error {
		minibus.Ready(ctx)

		if err := minibus.Send(ctx, "Hello, world!"); err != nil {
			return err
		}

		return minibus.Send(ctx, 42)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	if err := minibus.Run(
		ctx,
		minibus.WithFunc(recipient),
		minibus.WithFunc(sender),
	); err != context.DeadlineExceeded {
		fmt.Println(err)
	}

	// Output:
	// Hello, world!
	// 42
}
