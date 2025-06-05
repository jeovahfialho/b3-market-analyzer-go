FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git make gcc musl-dev

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s -X main.version=$(git describe --tags --always || echo 'dev')" \
    -a -installsuffix cgo -o b3-analyzer cmd/api/main.go

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" \
    -a -installsuffix cgo -o b3-analyzer-cli cmd/cli/main.go

FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata curl

RUN addgroup -g 1000 -S app && \
    adduser -u 1000 -S app -G app

WORKDIR /app

COPY --from=builder /build/b3-analyzer /app/
COPY --from=builder /build/b3-analyzer-cli /app/

COPY --from=builder /build/scripts /app/scripts

RUN mkdir -p /app/data && chown -R app:app /app

USER app

EXPOSE 8000

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8000/health || exit 1

CMD ["/app/b3-analyzer"]