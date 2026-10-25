FROM golang:1.25-bookworm AS builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    poppler-utils \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN curl -sL https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64 \
    -o /usr/local/bin/tailwindcss && chmod +x /usr/local/bin/tailwindcss

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN tailwindcss -i ./internal/web/static/input.css -o ./internal/web/static/app.css --minify
RUN go build -trimpath -ldflags='-s -w' -o /app/buh .

ENTRYPOINT ["./buh", "server"]

# dev stage — retains the full Go/Tailwind toolchain for hot reload.
FROM builder AS dev
# Air v1.67 requires Go 1.26; pin a release compatible with Go 1.25.
RUN go install github.com/air-verse/air@v1.66.1

# The default stage contains only the application and its runtime dependencies.
FROM debian:bookworm-slim AS production

RUN apt-get update && apt-get install -y --no-install-recommends \
    poppler-utils \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /app/buh ./buh

ENTRYPOINT ["./buh", "server"]
