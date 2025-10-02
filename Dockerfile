# Build stage
FROM golang:1.25.1-alpine AS builder

# Build arguments for multi-platform support
ARG TARGETOS
ARG TARGETARCH

# Install dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy go module files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application with platform-specific variables
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -ldflags="-s -w" \
    -o ecsazrlc \
    ./cmd

# Runtime stage
FROM alpine:3.18.12

# Install ca-certificates for HTTPS and Docker CLI for socket access
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/ecsazrlc .

# Create non-root user
RUN addgroup -g 1000 ecsazrlc && \
    adduser -D -u 1000 -G ecsazrlc ecsazrlc && \
    chown -R ecsazrlc:ecsazrlc /app

# Switch to non-root user
USER ecsazrlc

# Expose healthcheck port (optional)
# EXPOSE 8080

ENTRYPOINT ["/app/ecsazrlc"]
CMD ["--help"]
