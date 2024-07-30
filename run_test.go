package minibus_test

import (
	"context"
	"testing"
	"time"

	. "github.com/dogmatiq/minibus"
)

func TestRun(t *testing.T) {
	t.Run("it exits cleanly when there are no components", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		if err := Run(ctx); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
