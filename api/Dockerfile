FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy source code
COPY . .

# Build the service - optimized for speed
RUN CGO_ENABLED=0 GOOS=linux go build -o http-service ./main.go
#RUN CGO_ENABLED=0 GOOS=linux go build -o consumer-service ./consumer/main.go
# Build with optimization flags to reduce binary size by ~30-40%
RUN CGO_ENABLED=0 GOOS=linux go build \
FROM alpine:3.18

WORKDIR /app

# Final stage - minimal alpine image
FROM alpine:3.19
    bash \
    ca-certificates \
    curl \
# Install only essential runtime dependencies in one layer
RUN apk add --no-cache bash ca-certificates curl && \
    curl -L https://github.com/golang-migrate/migrate/releases/download/v4.18.3/migrate.linux-amd64.tar.gz \
    | tar xvz -C /usr/local/bin migrate && \
    chmod +x /usr/local/bin/migrate && \
    rm -rf /var/cache/apk/*
    && chmod +x /usr/local/bin/migrate \
# Copy only the compiled binary and required files
#COPY --from=builder /app/consumer-service .
COPY migration ./migration
COPY config ./config
COPY entrypoint.sh .

RUN chmod +x entrypoint.sh

# Run as non-root user for security
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser && \
    chown -R appuser:appuser /app

USER appuser

RUN chmod +x entrypoint.sh

EXPOSE 8080

ENTRYPOINT ["./entrypoint.sh"]
