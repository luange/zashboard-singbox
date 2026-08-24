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
  proxies the core API, exposes the four allow-listed supervisor operations,
  and applies declarative Provider overrides through one fixed config builder.
  It never executes user-supplied commands.

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
  -ui /usr/share/zashboard-singbox \
  -provider-overrides /etc/zashboard-controller/provider-overrides.json \
  -config-builder /usr/local/sbin/singbox-build-runtime-config
```

The service name is validated and passed only to systemd or OpenRC with one of
`start`, `stop`, or `restart`. Bind to loopback unless LAN access is required.
When listening on a LAN address, use a separate strong controller token.
For OpenRC, write the token as
`export ZASHBOARD_CONTROLLER_TOKEN=...` in
`/etc/conf.d/zashboard-controller`; plain sourced assignments are not exported
to the controller process. The packaged environment example already uses the
portable exported form.

Lifecycle actions are not acknowledged merely because the supervisor command
returned. The dashboard polls the independent controller until the core is
actually reachable after start/restart, or actually stopped after stop.

## Provider overrides

Upstream Zashboard can display, update, and health-check Clash-compatible
Providers, but it cannot create or edit subscription definitions. This edition
adds a general Provider override layer instead of editing the original sing-box
configuration:

- multiple remote or local Providers can be injected;
- an existing Provider can be partially overridden while unspecified fields
  continue to come from the base configuration;
- each Provider can be attached to zero or more Smart groups through
  `attach_to`;
- deleting an override restores the original Provider definition and group
  membership on the next validated restart;
- a newly supplied remote address must pass a bounded HTTP content check before
  the override is persisted;
- the merged runtime configuration is built and checked before the service is
  restarted.

The authenticated endpoints are:

- `GET /controller/v1/provider-overrides`
- `PUT /controller/v1/provider-overrides/{tag}`
- `DELETE /controller/v1/provider-overrides/{tag}`

The override file is atomically written with mode `0600`. GET responses never
return subscription URLs or headers; only `url_configured` and
`headers_configured` booleans are exposed. Query strings and fragments are
removed from displayed health-check URLs. Check failures return only a redacted
reason. The original configuration remains unchanged, so the feature is
reversible and survives subscription regeneration.

The dashboard must be opened through the independent controller address to use
editing and lifecycle controls. A dashboard served directly by the core keeps
ordinary Clash-compatible display/update behavior but cannot manage the host
service or its configuration files.

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
