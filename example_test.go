package minibus_test

import (
	"context"
	"fmt"

	"github.com/dogmatiq/minibus"
)

func Example() {
	// SayHello is an example message type. Minibus doesn't care what types you
	// use for messages, but it's typical to use struct types.
	type SayHello struct {
		Name string
	}

	// The recipient function implements a "component" that receives SayHello
	// messages.
	recipient := func(ctx context.Context) error {
		// First, we subscribe to the types of messages we're interested in.
		minibus.Subscribe[SayHello](ctx)

		// All components must be started before we can send or receive
		// messages.
		minibus.Start(ctx)

		// Finally, we receive messages from our component-specific inbox.
		//
		// The inbox channel is closed when ctx.Done() is closed, so it's not
		// necessary to select on both.
		for m := range minibus.Inbox(ctx) {
			switch m := m.(type) {
			case SayHello:
				fmt.Printf("Hello, %s!\n", m.Name)
				return nil // let's not wait for any more greetings
			}
		}

		// If the inbox is closed before we received our message it means we
		// were signaled to stop.
		return ctx.Err()
	}

	// The sender function implements a "component" that sends a SayHello
	// message.
	sender := func(ctx context.Context) error {
		// Start must be called by all components, even if they do not subscribe
		// to any message types.
		minibus.Start(ctx)

		// Then we say hello to the other component!
		return minibus.Send(ctx, SayHello{"world"})
	}

	// The Run function executes each component in its own goroutine and
	// delivers messages between them. It blocks until all component's
	// goroutines have exited.
	if err := minibus.Run(
		context.Background(),
		minibus.WithComponent(recipient),
		minibus.WithComponent(sender),
	); err != nil {
		fmt.Println(err)
	}

	// Output:
	// Hello, world!
}
