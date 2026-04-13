# Multi-stage build: static noteserver (no CGO; server does not use SQLite).
FROM golang:1.21-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /noteserver ./cmd/noteserver

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=build /noteserver /noteserver
VOLUME ["/data"]
EXPOSE 8080
# Runtime: set NOTE_ADMIN_PASSWORD, or pass -password=... (see image comment / docker run help).
ENTRYPOINT ["/noteserver"]
CMD ["-listen=:8080", "-datadir=/data"]
