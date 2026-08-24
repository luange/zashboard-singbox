# Zashboard sing-box edition

This edition keeps Zashboard usable with sing-box after upstream removes its
native integration. It is a general project: no provider names, group names,
filesystem paths, or host addresses are built into the UI.

## Architecture

- The existing Clash-compatible channel remains the common data plane.
- `/capabilities` is preferred over parsing version strings for optional
  features.
- Provider management uses the standard `/providers/proxies` resource:
  update, health check, recoverable disable/restore, and guarded permanent
  deletion.
- DNS and FakeIP maintenance operations are idempotent. Clearing an unused
  FakeIP store is a successful no-op.
- Smart is treated as a native group type. The UI consumes the core's current
  selection and Smart metadata rather than emulating URLTest behavior.
- `zashboard-controller` is an optional companion process. It serves the UI,
  proxies the core API, and exposes only four supervisor operations. It never
  executes user-supplied commands.

## Controller

The controller must run independently from sing-box; otherwise a stopped core
could not be started from the dashboard.

```sh
export ZASHBOARD_CONTROLLER_TOKEN='replace-with-a-long-random-token'
zashboard-controller \
  -listen 0.0.0.0:9091 \
  -core http://127.0.0.1:9090 \
  -service singbox \
  -supervisor auto \
  -ui /usr/share/zashboard-singbox
```

The service name is validated and passed only to systemd or OpenRC with one of
`start`, `stop`, or `restart`. Bind to loopback unless LAN access is required.
When listening on a LAN address, use a separate strong controller token.

## Provider lifecycle

- Update: `PUT /providers/proxies/{name}`
- Disable (recoverable): `DELETE /providers/proxies/{name}`
- Restore: `PATCH /providers/proxies/{name}` with `{"paused":false}`
- Pause: `PATCH /providers/proxies/{name}` with `{"paused":true}`
- Permanent delete: `DELETE /providers/proxies/{name}?permanent=true`

Recoverable disable keeps the last valid node set available but stops scheduled
downloads and health checks. Permanent deletion is rejected with HTTP 409 while
runtime consumers are registered.

## Following upstream Zashboard

The repository retains `https://github.com/Zephyruso/zashboard.git` as the
`upstream` remote. sing-box changes stay in small, domain-focused commits and
inside the API/assembly boundary. A scheduled workflow merges upstream `main`
in a disposable CI checkout and runs type checking plus a production build.
Conflicts fail visibly; they are never auto-pushed or auto-released.

Release branches are cut only after:

1. upstream compatibility job passes;
2. UI type-check and build pass;
3. controller tests and cross-builds pass;
4. sing-box API contract tests pass;
5. stop/start, provider disable/restore, DNS flush, and FakeIP flush are tested
   against an isolated instance.
