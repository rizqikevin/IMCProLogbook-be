# syntax=docker/dockerfile:1
FROM golang:1.26-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /bin/logbook \
    ./cmd/logbook

FROM alpine:3.23

RUN apk add --no-cache ca-certificates \
    && addgroup -g 10001 logbook \
    && adduser -D -u 10001 -G logbook logbook \
    && mkdir -p /app/data/uploads \
    && chown -R logbook:logbook /app

WORKDIR /app

COPY --from=builder /bin/logbook /bin/logbook

ENV APP_ENV=production \
    HTTP_ADDR=:8080

EXPOSE 8080

USER 10001:10001
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:8080/health/ready || exit 1

ENTRYPOINT ["/bin/logbook"]
CMD ["serve"]
