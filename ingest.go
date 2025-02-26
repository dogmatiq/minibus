package minibus

import "context"

// Ingest returns a [Func] that reads messages from the provided channel and
// forwards them to the bus.
func Ingest[T any](messages <-chan T) Func {
	return func(ctx context.Context) error {
		Ready(ctx)

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case m, ok := <-messages:
				if !ok {
					return nil
				}
				if err := Send(ctx, m); err != nil {
					return err
				}
			}
		}
	}
}
