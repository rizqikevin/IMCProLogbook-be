FROM node:24-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM golang:1.26-alpine AS server
WORKDIR /src
COPY server/main.go ./main.go
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /web main.go

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && addgroup -g 10001 web && adduser -D -u 10001 -G web web
WORKDIR /app
COPY --from=server /web /usr/local/bin/web
COPY --from=build /app/dist ./dist
USER 10001:10001
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:8080/health || exit 1
ENTRYPOINT ["/usr/local/bin/web"]
