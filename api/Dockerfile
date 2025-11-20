FROM golang:1.23-alpine AS builder
WORKDIR /gau_cloud

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o gau-cloud-service.bin .

FROM alpine:latest
WORKDIR /gau_cloud

RUN apk add --no-cache \
    bash \
    ca-certificates \
    curl \
    && curl -L https://github.com/golang-migrate/migrate/releases/download/v4.18.3/migrate.linux-amd64.tar.gz \
    | tar xvz -C /tmp \
    && mv /tmp/migrate /usr/local/bin/migrate \
    && chmod +x /usr/local/bin/migrate

COPY --from=builder /gau_cloud/gau-cloud-service.bin .
COPY migrations ./migrations
COPY entrypoint.sh .

RUN chmod +x entrypoint.sh

EXPOSE 8080

ENTRYPOINT ["./entrypoint.sh"]