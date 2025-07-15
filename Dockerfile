# Build stage
FROM golang:1.24-bookworm AS builder
WORKDIR /app
COPY . .
ENV CGO_ENABLED=1
RUN go build -ldflags="-s -w" -o uptime-monitor main.go

# Production stage
FROM debian:bookworm-slim
WORKDIR /app
RUN mkdir -p /app/data
COPY --from=builder /app/uptime-monitor /app/uptime-monitor
COPY --from=builder /app/static /app/static
COPY --from=builder /app/data/uptime.db /app/data/uptime.db
EXPOSE 8080
VOLUME ["/app/data"]
ENV SQLITE_DB=/app/data/uptime.db
CMD ["/app/uptime-monitor"]
