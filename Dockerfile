FROM golang:1.22-bookworm

RUN apt-get update && apt-get install -y --no-install-recommends \
    poppler-utils \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY go.mod ./
COPY main.go ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/

RUN go get github.com/makiuchi-d/gozxing \
    && go get github.com/skip2/go-qrcode \
    && go get github.com/spf13/cobra \
    && go mod tidy \
    && go build -o buh .

ENTRYPOINT ["./buh"]
