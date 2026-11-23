<div align="center">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="https://dogmatiq.io/res/img/dynamic/repo/minibus/banner/dark.svg">
  <img alt="Dogmatiq minibus" src="https://dogmatiq.io/res/img/dynamic/repo/minibus/banner/light.svg" height="44">
</picture>

Minibus is a very small in-memory message bus for Go.

[![API reference](https://dogmatiq.io/res/img/dynamic/repo/minibus/badge/docs.svg)](https://pkg.go.dev/github.com/dogmatiq/minibus)
[![Latest version](https://dogmatiq.io/res/img/dynamic/repo/minibus/badge/semver.svg)](https://github.com/dogmatiq/minibus/releases)
[![Build status](https://dogmatiq.io/res/img/dynamic/repo/minibus/badge/build.svg)](https://github.com/dogmatiq/minibus/actions/workflows/ci.yml)
[![Code coverage](https://dogmatiq.io/res/img/dynamic/repo/minibus/badge/coverage.svg)](https://codecov.io/github/dogmatiq/minibus)

</div>

Minibus executes a set of functions concurrently and exchanges messages between
them. You can think of it like an [`errgroup.Group`] with a built-in message
bus.

## Example

A more detailed version of this example annotated with comments is available
[here](example_test.go).

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
