package minibus_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/dogmatiq/minibus"
)

func Example() {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if err := Run(
		ctx,
		WithComponent(func(ctx context.Context) error {
			Subscribe[string](ctx)
			Start(ctx)

			m, err := Receive(ctx)
			if err != nil {
				return err
			}

			fmt.Println("received:", m)
			return nil
		}),
		WithComponent(func(ctx context.Context) error {
			Start(ctx)
			return Send(ctx, "hello")
		}),
	); err != nil {
		fmt.Println(err)
	}

	// Output:
	// received: hello
}
