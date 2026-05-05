FROM golang:1.26 AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/payment-service ./cmd/payment-service

FROM scratch
COPY --from=builder /out/payment-service /usr/local/bin/payment-service
EXPOSE 8081 50051
ENTRYPOINT ["/usr/local/bin/payment-service"]
