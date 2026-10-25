# Production container runtime image

## Problem

The production container included the Go and Tailwind build toolchains and the source tree, despite the application embedding its runtime assets.

## Decision and approach

Use a multi-stage Dockerfile with a full Go/Tailwind builder and preserve the `dev` target for Air-based hot reload. Build a stripped binary and run it from `debian:bookworm-slim` with only Poppler, curl, and CA certificates installed. Reject Alpine to retain conservative Debian/glibc compatibility for native runtime utilities.

## Implemented

The default production stage now copies only the built `buh` binary; embedded templates, static assets, and locales require no runtime files. The production entrypoint remains `./buh server`.

## Validation

Go tests, both Docker build targets, image size inspection, and whitespace checks were run for this change.
