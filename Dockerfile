# =============================================================================
# Dockerfile for iDRAC NetBox Importer
# Optimized for GitLab CI/CD Pipeline
# =============================================================================
# Build: docker build -t idrac-inventory .
# Run:   docker run --rm idrac-inventory -version
# =============================================================================

FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# Copy dependency files first (better caching)
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build arguments (set by CI/CD pipeline)
ARG VERSION=dev
ARG BUILD_TIME=unknown
ARG GIT_COMMIT=unknown

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT}" \
    -o idrac-inventory ./cmd/idrac-inventory

# =============================================================================
# Runtime Image - Minimal and Secure
# =============================================================================
FROM alpine:3.21

# Install runtime dependencies only
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 1000 appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/idrac-inventory /app/idrac-inventory
COPY --from=builder /build/config.yaml /app/config.yaml.example

# Set ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user for security
USER appuser

# Default environment variables (override with -e in docker run or CI/CD variables)
ENV IDRAC_LOG_LEVEL=info \
    IDRAC_LOG_FORMAT=json \
    IDRAC_CONCURRENCY=5

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ["/app/idrac-inventory", "-version"]

ENTRYPOINT ["/app/idrac-inventory"]
CMD ["-help"]
