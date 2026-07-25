# Dynamic rules security model

Rules are administrator-managed code, not a multi-tenant isolation boundary.
They run in a fresh Goja VM for every request and receive only a frozen native
JavaScript request object. The engine deliberately exposes no Go values,
`require`, `process`, filesystem, network, environment, subprocess, or HTTP
bindings. TypeScript/JavaScript is transformed with esbuild using its in-memory
Go API; no Node.js process is started.

The engine enforces byte limits for source, transformed code, canonical request
input, and decision output. It also has a bounded rule count and interrupts
JavaScript execution when either the supplied context expires or the configured
execution duration elapses. Failed evaluation returns a block decision together
with an error; callers must not turn that error into an allow.

Some boundaries cannot be made hard inside an in-process JavaScript runtime:

- Goja has no per-runtime heap quota. A rule can allocate memory before the
  result limit is checked. Run the agent with an OS/container memory limit.
- Goja interruption applies while JavaScript is executing. It cannot preempt a
  native Go builtin already running, and esbuild transformation during reload is
  likewise not context-cancellable. Source-size limits keep that exposure
  bounded but do not create a hard wall-clock compile limit.
- The caller owns HTTP body capture. `RequestFromHTTP` never reads `r.Body`; it
  is the integration point's responsibility to cap the bytes it provides.
