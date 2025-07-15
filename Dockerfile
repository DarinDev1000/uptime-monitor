# Build stage
FROM golang:1.20-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -ldflags="-s -w" -o uptime-monitor main.go

# Production stage
FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/uptime-monitor /app/uptime-monitor
COPY --from=builder /app/static /app/static
COPY --from=builder /app/uptime.db /app/uptime.db
EXPOSE 8080
CMD ["/app/uptime-monitor"]
