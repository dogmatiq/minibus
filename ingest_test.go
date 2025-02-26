package minibus_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/dogmatiq/minibus"
)

func TestRun_ingest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	expect := []string{"one", "two", "three"}
	messages := make(chan string, len(expect))
	for _, m := range expect {
		messages <- m
	}
	close(messages)

	err := Run(
		ctx,
		Ingest(messages),
		func(ctx context.Context) error {
			Subscribe[string](ctx)
			Ready(ctx)

			for _, want := range expect {
				got, err := Receive(ctx)
				if err != nil {
					return err
				}
				if got != want {
					return fmt.Errorf("unexpected message: got %q, want %q", got, want)
				}
			}

			return nil
		},
	)

	if err != nil {
		t.Fatalf("Run() returned an unexpected error: %s", err)
	}
}
