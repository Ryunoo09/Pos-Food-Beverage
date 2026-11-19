# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X main.version=${VERSION}" \
    -o /bin/server ./cmd/server

# Runtime stage
FROM alpine:3.21 AS production

# pg_dump untuk backup F21, openssl untuk enkripsi, ca-certificates untuk HTTPS keluar.
RUN apk add --no-cache ca-certificates openssl postgresql-client tzdata \
    && adduser -D -u 10001 app \
    && mkdir -p /backups \
    && chown app:app /backups

COPY --from=builder /bin/server /server

USER app
EXPOSE 8080
ENTRYPOINT ["/server"]
