# ADR 012: Production container runtime image

## Context

The application image currently includes the Go toolchain, Tailwind CLI, source tree, and build dependencies at runtime. This makes the production image substantially larger than necessary. Development still uses the `dev` target with Go, Tailwind, and Air for hot reload. The application invokes Poppler utilities and needs outbound HTTPS and the Compose healthcheck, so its runtime image must retain those dependencies.

## Alternatives considered

1. **Retain the Go base image** — requires no structural change, but ships the compiler, build tooling, and source files in production.
2. **Debian slim multi-stage build** — keeps a full Go/Tailwind builder and development target, then copies only the binary into a small glibc-based runtime with the required utilities.
3. **Alpine runtime** — would generally be smaller, but introduces musl compatibility differences for a production application that invokes native utilities. It was rejected in favor of conservative Debian/glibc compatibility.

## Decision

Use a multi-stage Dockerfile. The `builder` stage retains the full Go and Tailwind build environment, and `dev` continues to derive from it and install Air. The default production stage is `debian:bookworm-slim`, installs only `poppler-utils`, `curl`, and `ca-certificates`, and copies only the stripped application binary. Templates, static assets, and locales remain embedded in that binary.

## Consequences

Production images are substantially smaller and contain no compiler, Tailwind CLI, source tree, or Go module cache. The runtime retains the native PDF tooling and HTTPS/healthcheck support. Development keeps its existing toolchain and hot-reload behavior. Production builds depend on the Debian slim package repositories and the build stage's Tailwind download.
