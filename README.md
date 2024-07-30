<div align="center">

# Minibus

Minibus is a very small in-memory message bus for Go.

[![Documentation](https://img.shields.io/badge/go.dev-documentation-007d9c?&style=for-the-badge)](https://pkg.go.dev/github.com/dogmatiq/minibus)
[![Latest Version](https://img.shields.io/github/tag/dogmatiq/minibus.svg?&style=for-the-badge&label=semver)](https://github.com/dogmatiq/minibus/releases)
[![Build Status](https://img.shields.io/github/actions/workflow/status/dogmatiq/minibus/ci.yml?style=for-the-badge&branch=main)](https://github.com/dogmatiq/minibus/actions/workflows/ci.yml)
[![Code Coverage](https://img.shields.io/codecov/c/github/dogmatiq/minibus/main.svg?style=for-the-badge)](https://codecov.io/github/dogmatiq/minibus)

</div>

Minibus executes a set of functions concurrently and exchanges messages between
them. You can think of it like an [`errgroup.Group`] with a built-in message
bus.

```go
type SayHello struct {
    Name string
}

minibus.Run(
    context.Background(),
    minibus.WithFunc(
        func(ctx context.Context) error {
            minibus.Subscribe[SayHello](ctx)
            minibus.Ready(ctx)

            for m := range minibus.Inbox(ctx) {
                switch m := m.(type) {
                case SayHello:
                    fmt.Printf("Hello, %s!\n", m.Name)
                }
            }

            return nil
        },
    ),
    minibus.WithFunc(
        func(ctx context.Context) error {
            minibus.Ready(ctx)
            return minibus.Send(ctx, SayHello{"world"})
        },
    ),
)
```

[`errgroup.Group`]: https://pkg.go.dev/golang.org/x/sync/errgroup#Group
