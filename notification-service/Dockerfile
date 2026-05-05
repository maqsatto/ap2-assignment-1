FROM golang:1.26 AS builder
WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/notification-service ./cmd/notification-service

FROM scratch
COPY --from=builder /out/notification-service /usr/local/bin/notification-service
ENTRYPOINT ["/usr/local/bin/notification-service"]
