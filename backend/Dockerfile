# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/server ./cmd/server

# Runtime stage
FROM alpine:3.21

RUN adduser -D -u 10001 app
COPY --from=builder /bin/server /server

USER app
EXPOSE 8080
ENTRYPOINT ["/server"]
