// Package dynamicrules evaluates administrator-managed JavaScript and
// TypeScript request rules in a capability-free Goja runtime.
//
// A rule must export either evaluate or a default function. The function gets
// an immutable, deterministic request object and returns either null (no
// decision) or a plain object with an explicit action field:
//
//	{ action: "allow", reason?: string }
//	{ action: "block", reason?: string }
//
// Rules are transformed with esbuild before they are loaded. They do not
// receive require, process, filesystem, network, Go objects, or any other host
// capability. Each evaluation uses a fresh VM, so mutable JavaScript state
// cannot leak from one request to another.
//
// The package enforces bounded source, compiled-code, request-input, result,
// rule-count, and execution-time limits. See README.md for important limits
// that a Go in-process runtime cannot hard-enforce, including heap use.
package dynamicrules
