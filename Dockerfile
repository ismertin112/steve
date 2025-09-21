# syntax=docker/dockerfile:1
FROM golang:1.21-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o vpn-bot ./cmd/bot

FROM alpine:3.18
WORKDIR /app
COPY --from=build /app/vpn-bot ./vpn-bot
COPY migrations ./migrations
ENV TELEGRAM_TOKEN="" \
    ADMIN_IDS="" \
    PANEL_URL="" \
    PANEL_TOKEN="" \
    DB_DSN=""
CMD ["./vpn-bot"]
