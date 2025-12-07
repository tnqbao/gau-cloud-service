# stage: builder
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

WORKDIR /app
ENV GOPROXY=https://proxy.golang.org,direct

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -ldflags="-w -s" -o http-server.bin ./http/main.go

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -ldflags="-w -s" -o consumer-worker.bin ./consumer/consumer_main.go


# stage: migrator
FROM alpine:3.18 AS migrator

RUN apk add --no-cache curl \
    && curl -L https://github.com/golang-migrate/migrate/releases/download/v4.18.3/migrate.linux-amd64.tar.gz \
       -o /tmp/migrate.tar.gz \
    && tar -xzf /tmp/migrate.tar.gz -C /tmp \
    && chmod +x /tmp/migrate \
    && rm -rf /var/cache/apk/* /tmp/*.tar.gz


# stage: runtime
FROM alpine:3.18

WORKDIR /app

RUN apk add --no-cache bash ca-certificates tzdata

RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

COPY --from=builder /app/http-server.bin .
COPY --from=builder /app/consumer-worker.bin .
COPY --from=migrator /tmp/migrate /usr/local/bin/migrate

COPY --chown=appuser:appuser migration ./migration
COPY --chown=appuser:appuser config ./config
COPY --chown=appuser:appuser entrypoint.sh .
RUN chmod +x entrypoint.sh

USER appuser

EXPOSE 8080

ENTRYPOINT ["./entrypoint.sh"]
