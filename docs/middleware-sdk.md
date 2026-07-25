# Middleware SDK v1

NetGoat's middleware SDK is for **trusted code compiled into the agent
binary**. It is not a loader for native shared objects and it is not a sandbox
for tenant-supplied code. Use `dynamic_rules` for administrator-authored
JavaScript or TypeScript request policy instead.

The v1 contract is `netgoat.dev/middleware/v1`. A plugin declares its name,
version, API version, and the least set of capabilities it needs. The host must
both allow and explicitly grant every requested capability before the plugin is
started.

```go
package example

import (
    "context"
    "netgoat.xyz/agent/internal/middleware"
)

type maintenanceGate struct{}

func (maintenanceGate) Manifest() middleware.Manifest {
    return middleware.Manifest{
        Name: "maintenance-gate", Version: "1.0.0",
        APIVersion: middleware.APIVersion,
        Capabilities: []middleware.Capability{
            middleware.CapabilityRequestRead,
        },
    }
}

func (maintenanceGate) Start(context.Context) error { return nil }
func (maintenanceGate) Stop(context.Context) error  { return nil }

func (maintenanceGate) Handle(_ context.Context, request middleware.Request) (middleware.Decision, error) {
    if request.Path == "/maintenance" {
        return middleware.Decision{Action: middleware.ActionBlock}, nil
    }
    return middleware.Decision{Action: middleware.ActionAllow}, nil
}
```

An embedding application creates a registry with the capabilities it permits,
registers its compiled-in plugin, starts it before accepting traffic, and wraps
the appropriate handler:

```go
registry := middleware.NewRegistry(middleware.CapabilityRequestRead)
if err := registry.Register(maintenanceGate{}, middleware.CapabilityRequestRead); err != nil {
    return err
}
if err := registry.Start(ctx); err != nil {
    return err
}
defer registry.Close(context.Background())

handler := registry.Handler(next)
```

To make a factory selectable from application configuration, register it in
the application at setup time and load an enabled `middleware.Spec`. Factories
are intentionally registered in Go code, so the binary author controls exactly
which implementations can run.

```go
loader := middleware.NewLoader()
loader.RegisterFactory("maintenance-gate", func(raw json.RawMessage) (middleware.Plugin, error) {
    return maintenanceGate{}, nil
})
registry := middleware.NewRegistry(middleware.CapabilityRequestRead)
if err := registry.LoadAndRegister(loader, specs); err != nil {
    return err
}
```

`request.read` provides a copy of method, host, path, query, headers, and the
host-supplied client address. `route.read` provides only the resolved route
key. `response.write` is required before a blocking plugin can customize its
status, headers, or body. An allowed plugin cannot gain an ungranted capability
through its configuration.

Plugins execute in registration order. Startup failures roll back already
started plugins, shutdown runs in reverse order, and plugin errors or panics
fail the request closed with HTTP 500. Request headers are copied for every
invocation. Keep handlers fast, avoid holding request bodies, and use the
request context for cancellation.
