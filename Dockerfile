FROM golang:alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the executable
RUN CGO_ENABLED=0 GOOS=linux go build -o artha-icons ./cmd/sync/...

FROM alpine:latest

# Install CA certificates to enable outbound HTTPS requests (essential for downloading logos)
RUN apk --no-cache add ca-certificates

WORKDIR /app
COPY --from=builder /app/artha-icons .

# We'll map volumes for logos and data in docker-compose
VOLUME ["/app/logos", "/app/data"]

ENTRYPOINT ["./artha-icons"]
