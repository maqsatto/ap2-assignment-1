FROM golang:1.26 AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/order-service ./cmd/order-service

FROM scratch
COPY --from=builder /out/order-service /usr/local/bin/order-service
EXPOSE 8080 50052
ENTRYPOINT ["/usr/local/bin/order-service"]
