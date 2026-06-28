FROM golang:1.25-bookworm AS base

RUN apt-get update && apt-get install -y --no-install-recommends \
    poppler-utils \
    curl \
    && rm -rf /var/lib/apt/lists/*

RUN curl -sL https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64 \
    -o /usr/local/bin/tailwindcss && chmod +x /usr/local/bin/tailwindcss

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN tailwindcss -i ./internal/web/static/input.css -o ./internal/web/static/app.css --minify

RUN go build -o buh .

ENTRYPOINT ["./buh", "server"]

# dev stage — adds hot-reload tooling
FROM base AS dev
RUN go install github.com/air-verse/air@latest

# default stage for production builds
FROM base
