# Developer plugin catalog

The NetGoat developer catalog is a **selection and trust layer** around
middleware that is already compiled into an agent binary. It is intentionally
not a remote code-execution system: the agent does not download packages,
shared objects, WASM modules, JavaScript source, URLs, or artifacts from the
catalog.

This lets a publisher release metadata and configuration in the developer
portal while keeping the request path portable and auditable. A publisher's
verification status or credibility score may help operators decide whether to
install a release, but neither changes what an agent is permitted to execute.

## Selection contract

The control plane publishes a top-level restart-time snapshot section:

```json
{
  "plugins_configured": true,
  "plugins": {
    "installations": [
      {
        "plugin_id": "publisher-slug/plugin-slug",
        "factory_id": "compiled.factory-id",
        "version": "1.0.0",
        "sha256": "lowercase-64-character-descriptor-fingerprint",
        "api_version": "netgoat.dev/middleware/v1",
        "granted_capabilities": ["request.read"],
        "config": { "mode": "observe" }
      }
    ]
  }
}
```

`plugin_id` is the canonical catalog package ID: `${publisher.slug}/${plugin.slug}`.
The agent accepts an installation only when its package ID, factory ID, version,
`sha256`, API version, and complete capability set exactly match one descriptor
compiled into the running binary. Grant ordering is normalized, but duplicate,
unknown, missing, or extra grants are rejected.

`sha256` is the immutable **compiled descriptor fingerprint** supplied by an
agent release (`descriptor_sha256` in the developer catalog). It is not the
marketplace manifest/audit hash and not the digest of a downloaded file—there
is no downloaded file to verify. Keep a separate manifest hash for release
auditability if the portal records one.

The agent limits selections to 64 installations, each identity field to 128
bytes, factory configuration to a JSON object of at most 16 KiB, and all
configuration together to 256 KiB. `config` is passed only to the matched
compiled factory; the generic catalog runtime never interprets it as code.

## Restart-only behavior

Plugin selections are resolved during startup from local `config.yml` or the
last persisted control-plane snapshot. An arriving stream update is validated,
persisted, and logged as **restart required**. It never mutates an in-flight
registry or loads a new factory in a serving process.

Valid plugins run after NetGoat has resolved the trusted client IP and the
target route, so `request.read` and `route.read` get the intended metadata.
They run before traffic shaping and proxying. Login and verification endpoints
are deliberately outside this middleware extension point.

## Built-in reference descriptor

This build ships one harmless reference plugin to exercise the full catalog
path. It does not inspect requests, has no capabilities, and always allows the
request.

| Field | Value |
| --- | --- |
| `plugin_id` | `netgoat/noop` |
| `factory_id` | `builtin.noop` |
| `version` | `1.0.0` |
| `sha256` / `descriptor_sha256` | `22d6f9898bf456265c2549c38ef185c11de1da1eeee944bb8e97470ed438d2c0` |
| `api_version` | `netgoat.dev/middleware/v1` |
| `granted_capabilities` | `[]` |

Local configuration can select it explicitly:

```yaml
plugins:
  installations:
    - plugin_id: netgoat/noop
      factory_id: builtin.noop
      version: 1.0.0
      sha256: 22d6f9898bf456265c2549c38ef185c11de1da1eeee944bb8e97470ed438d2c0
      api_version: netgoat.dev/middleware/v1
      granted_capabilities: []
      config: {}
```

## Adding a real trusted extension

Publishing a catalog release alone never makes code executable. To support a
new runtime extension, the agent maintainer must:

1. Implement and test the middleware in Go using the v1 SDK.
2. Register its factory and immutable descriptor in agent source.
3. Release an agent binary containing that descriptor.
4. Publish the exact descriptor fingerprint to the catalog release.
5. Let an operator select that release and restart the agent.

The middleware SDK's lifecycle, capability, and failure behavior is documented
in [middleware-sdk.md](middleware-sdk.md). Use `dynamic_rules` for bounded
administrator-authored JavaScript/TypeScript request policy; do not treat a
developer catalog entry as a route for arbitrary code execution.
